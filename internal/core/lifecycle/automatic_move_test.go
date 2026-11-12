package lifecycle_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestSweepIndexPublication(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"ordinary", "same-size-time", "split", "split-many", "unborn", "worktree", "submodule"} {
		t.Run(mode, func(t *testing.T) {
			// Index generations and moves stay real; only publication order
			// matters here, so runner stalls must not expire Git authority.
			synctest.Test(t, func(t *testing.T) {
				f := newFixture(t)
				gitAt := func(root string, args ...string) string {
					t.Helper()
					out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
					if err != nil {
						t.Fatalf("git %v: %s %v", args, out, err)
					}
					return strings.TrimSpace(string(out))
				}
				git := func(args ...string) string { return gitAt(f.dir, args...) }
				if mode == "worktree" || mode == "submodule" {
					origin := t.TempDir()
					gitAt(origin, "init", "-q")
					gitAt(origin, "commit", "--allow-empty", "-qm", "seed")
					if mode == "worktree" {
						gitAt(origin, "worktree", "add", "-q", "--detach", f.dir)
					} else {
						git("init", "-q")
						git("-c", "protocol.file.allow=always", "submodule", "add", "-q", origin, "child")
						f.dir = filepath.Join(f.dir, "child")
					}
				} else {
					git("init", "-q")
				}
				paths := []string{f.file("first.log"), f.file("second.log"), f.file("third.log")}
				f.file("shadow.log")
				index := git("rev-parse", "--path-format=absolute", "--git-path", "index")
				var original []byte
				if mode != "unborn" {
					git("add", "--", "shadow.log")
					if mode == "split" || mode == "split-many" {
						git("config", "core.splitIndex", "true")
						git("update-index", "--split-index")
						if mode == "split-many" {
							for i := 0; i < 9; i++ {
								name := fmt.Sprintf("seed-%02d.txt", i)
								f.file(name)
								git("add", "--", name)
								git("update-index", "--split-index")
							}
						}
						if git("rev-parse", "--shared-index-path") == "" {
							t.Fatal("split-index fixture has no shared file")
						}
					}
					original = []byte(read(t, index))
					git("update-index", "--force-remove", "shadow.log")
				}
				git("add", "--", "second.log")
				staged := filepath.Join(f.dir, "staged-index")
				if err := os.Rename(index, staged); err != nil {
					t.Fatal(err)
				}
				if mode != "unborn" {
					if err := os.WriteFile(index, original, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "same-size-time" {
					old, _ := os.Stat(index)
					next, _ := os.Stat(staged)
					if old.Size() != next.Size() {
						t.Fatalf("index sizes differ: %d %d", old.Size(), next.Size())
					}
					for _, path := range []string{index, staged} {
						if err := os.Chtimes(path, old.ModTime(), old.ModTime()); err != nil {
							t.Fatal(err)
						}
					}
				}
				f.svc.Owner, f.svc.OwnerBatch = auto.Owner, auto.OwnerBatch
				f.svc.PrepareOwners = auto.PrepareOwners
				if rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
					t.Fatalf("add: %+v %v", rows, err)
				}
				f.clock = f.clock.Add(time.Hour)
				published := false
				restore := core.SetBurialStep(f.svc, func(step string) error {
					if step == "moved" && !published {
						published = true
						return os.Rename(staged, index)
					}
					return nil
				})
				defer restore()
				rows, err := f.svc.Sweep(false)
				if !published || git("ls-files", "--", "second.log") != "second.log" {
					t.Fatal("index publication fixture not reached")
				}
				if data, readErr := os.ReadFile(paths[1]); readErr != nil || string(data) != "second.log" {
					t.Fatalf("changed Git index authorized a stale move: %+v %v", rows, readErr)
				}
				if err != nil || len(rows) != 3 || rows[0].Action != core.ActionBuried || rows[1].Action != core.ActionRefused || rows[2].Action != core.ActionBuried || len(f.burials()) != 2 || len(f.items()) != 0 {
					t.Fatalf("index publication affected unrelated siblings: %+v %v", rows, err)
				}
			})
		})
	}
}

func TestSweepRejectsUnverifiableIndexSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"shared-replaced", "snapshot-error"} {
		t.Run(mode, func(t *testing.T) {
			// Reach the real index change after the first move without a
			// scheduling delay substituting an earlier timeout for rejection.
			synctest.Test(t, func(t *testing.T) {
				f := newFixture(t)
				git := func(args ...string) string {
					t.Helper()
					out, err := exec.Command("git", append([]string{"-C", f.dir}, args...)...).CombinedOutput()
					if err != nil {
						t.Fatalf("git: %s %v", out, err)
					}
					return strings.TrimSpace(string(out))
				}
				git("init", "-q")
				f.file("seed.txt")
				git("add", "--", "seed.txt")
				var shared string
				if mode == "shared-replaced" {
					git("update-index", "--split-index")
					shared = git("rev-parse", "--path-format=absolute", "--shared-index-path")
					if shared == "" {
						t.Fatal("missing shared-index fixture")
					}
				}
				paths := []string{f.file("first.log"), f.file("keep.log")}
				changed := false
				f.svc.Owner, f.svc.OwnerBatch = auto.Owner, auto.OwnerBatch
				f.svc.PrepareOwners = func(ctx context.Context, roots []string) core.OwnerBatchFunc {
					query := auto.PrepareOwners(ctx, roots)
					return func(ctx context.Context, paths []string) []core.Ownership {
						owners := query(ctx, paths)
						for i := range owners {
							current := owners[i].Current
							if current != nil {
								owners[i].Current = func(ctx context.Context) (bool, time.Time, error) {
									if changed && mode == "snapshot-error" {
										return false, time.Now(), os.ErrPermission
									}
									return current(ctx)
								}
							}
						}
						return owners
					}
				}
				if rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
					t.Fatalf("add: %+v %v", rows, err)
				}
				f.clock = f.clock.Add(time.Hour)
				restore := core.SetBurialStep(f.svc, func(step string) error {
					if step != "moved" || changed {
						return nil
					}
					changed = true
					if mode == "shared-replaced" {
						if err := os.WriteFile(shared+".replacement", []byte("broken index"), 0600); err != nil {
							return err
						}
						return os.Rename(shared+".replacement", shared)
					}
					return nil
				})
				defer restore()
				rows, err := f.svc.Sweep(false)
				if data, readErr := os.ReadFile(paths[1]); readErr != nil || string(data) != "keep.log" {
					t.Fatalf("unverifiable index snapshot moved content: %+v %v", rows, readErr)
				}
				if err != nil || !changed || len(rows) != 2 || rows[1].Action != core.ActionRetry || len(f.items()) != 1 || len(f.burials()) != 1 {
					t.Fatalf("unverifiable snapshot lost retry: %+v %v", rows, err)
				}
			})
		})
	}
}

func TestSweepBatchesStableGitAnswers(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	// This checks query reuse and per-move validation, not elapsed budgets.
	// Real Git and index checks remain; interval samples use the bubble clock.
	synctest.Test(t, func(t *testing.T) {
		f := newFixture(t)
		testenv.InitRepo(t, f.dir)
		paths := make([]string, 32)
		for i := range paths {
			paths[i] = f.file(fmt.Sprintf("file-%02d.log", i))
		}
		calls, moves := 0, 0
		var checkedAt time.Time
		var queriedAt time.Time
		var longest, rawLongest time.Duration
		f.svc.PrepareOwners = func(ctx context.Context, roots []string) core.OwnerBatchFunc {
			query := auto.PrepareOwners(ctx, roots)
			return func(ctx context.Context, paths []string) []core.Ownership {
				calls++
				if !ledgerUnlocked(t, f) {
					t.Fatal("snapshot ls-files held ledger lock")
				}
				owners := query(ctx, paths)
				for i := range owners {
					current := owners[i].Current
					queryTime := owners[i].CheckedAt
					if current != nil {
						owners[i].Current = func(ctx context.Context) (bool, time.Time, error) {
							valid, started, err := current(ctx)
							checkedAt = started
							queriedAt = queryTime
							return valid, started, err
						}
					}
				}
				return owners
			}
		}
		f.fs.onRecycle = func(string) {
			moves++
			longest = max(longest, time.Since(checkedAt))
			rawLongest = max(rawLongest, time.Since(queriedAt))
		}
		if rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
			t.Fatalf("add: %+v %v", rows, err)
		}
		f.clock = f.clock.Add(time.Hour)
		rows, err := f.svc.Sweep(false)
		if err != nil || len(f.burials()) != len(paths) {
			t.Fatalf("stable index sweep incomplete: %+v %v", rows, err)
		}
		if calls > 1+(len(paths)+31)/32 {
			t.Fatalf("stable index restarted Git per move: %d queries for %d paths", calls, len(paths))
		}
		if moves != len(paths) || checkedAt.IsZero() {
			t.Fatalf("index generation was not checked for moves: %d", moves)
		}
		t.Logf("moves=%d Git batches=%d maximum index-validation-to-MovePinned-entry=%s maximum reused-batch-age=%s", moves, calls, longest, rawLongest)
	})
}

func TestSweepRefreshesExpiredGitAnswer(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", f.dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	git("init", "-q")
	path := f.file("keep.log")
	f.svc.Owner, f.svc.OwnerBatch = auto.Owner, auto.OwnerBatch
	f.svc.PrepareOwners = auto.PrepareOwners
	if rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Hour)
	git("add", "--", "keep.log")
	index, staged := filepath.Join(f.dir, ".git", "index"), filepath.Join(f.dir, "staged-index")
	if err := os.Rename(index, staged); err != nil {
		t.Fatal(err)
	}
	published := false
	restore := core.SetBurialStep(f.svc, func(step string) error {
		if step == "git-checked" && !published {
			if err := os.Rename(staged, index); err != nil {
				t.Fatal(err)
			}
			published = true
			time.Sleep(20 * time.Millisecond)
		}
		return nil
	})
	defer restore()
	out, err := f.svc.Sweep(false)
	if err != nil || !published {
		t.Fatalf("stale-answer fixture not reached: %+v %v", out, err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "keep.log" {
		t.Fatalf("expired Git answer moved newly tracked file: %+v %v", out, readErr)
	}
	if len(f.burials()) != 0 {
		t.Fatal("stale answer created a burial")
	}
}

func TestSweepGitConfigurationPublication(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	// Real Git publishes and rechecks the configuration between moves;
	// scheduler time must not hide that change behind a query-budget retry.
	synctest.Test(t, func(t *testing.T) {
		f := newFixture(t)
		git := func(args ...string) string {
			t.Helper()
			out, err := exec.Command("git", append([]string{"-C", f.dir}, args...)...).CombinedOutput()
			if err != nil {
				t.Fatalf("git: %s %v", out, err)
			}
			return strings.TrimSpace(string(out))
		}
		git("init", "-q")
		f.file("work/keep.log")
		git("add", "--", "work/keep.log")
		paths := []string{f.file("first.log"), f.file("keep.log"), f.file("third.log")}
		f.svc.Owner, f.svc.OwnerBatch, f.svc.PrepareOwners = auto.Owner, auto.OwnerBatch, auto.PrepareOwners
		if rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
			t.Fatalf("add: %+v %v", rows, err)
		}
		f.clock = f.clock.Add(time.Hour)
		changed := false
		restore := core.SetBurialStep(f.svc, func(step string) error {
			if step == "moved" && !changed {
				changed = true
				git("config", "core.worktree", filepath.Dir(f.dir))
			}
			return nil
		})
		defer restore()
		rows, err := f.svc.Sweep(false)
		if !changed || git("ls-files", "--", "keep.log") != "keep.log" {
			t.Fatal("configuration publication fixture not reached")
		}
		if data, readErr := os.ReadFile(paths[1]); readErr != nil || string(data) != "keep.log" {
			t.Fatalf("changed Git configuration authorized a stale move: %+v %v", rows, readErr)
		}
		if err != nil || len(rows) != 3 || rows[1].Action != core.ActionRefused || len(f.burials()) != 2 || len(f.items()) != 0 {
			t.Fatalf("configuration recheck blocked unrelated siblings: %+v %v", rows, err)
		}
	})
}

func TestSweepSlowGitProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repositories and delayed queries")
	}
	for _, snapshot := range []bool{true, false} {
		t.Run(fmt.Sprint(snapshot), func(t *testing.T) {
			f, paths, _ := slowGitFixture(t, "scratch.log")
			calls, unlocked := 0, 0
			f.svc.PrepareOwners = func(ctx context.Context, paths []string) core.OwnerBatchFunc {
				query := auto.OwnerBatch
				if snapshot {
					query = auto.PrepareOwners(ctx, paths)
				}
				return func(ctx context.Context, paths []string) []core.Ownership {
					calls++
					if ledgerUnlocked(t, f) {
						unlocked++
					}
					return delayedOwners(ctx, query, paths)
				}
			}
			for attempt := 1; attempt <= 3; attempt++ {
				rows, err := f.svc.Sweep(false)
				if err != nil {
					t.Fatal(err)
				}
				items := f.items()
				if len(items) == 0 {
					if len(f.burials()) != 1 || exists(paths[0]) || unlocked == 0 {
						t.Fatalf("slow Git did not bury outside-lock verified content: %+v", rows)
					}
					t.Logf("350ms injected startup: attempts=%d queries=%d unlocked=%d", attempt, calls, unlocked)
					return
				}
				t.Logf("attempt=%d attempts=%d next=%s lastError=%q", attempt, items[0].Attempts, items[0].NextAttempt.Sub(f.clock), items[0].LastError)
				f.clock = items[0].NextAttempt
			}
			t.Fatalf("slow Git never progressed: queries=%d unlocked=%d items=%+v", calls, unlocked, f.items())
		})
	}
}

func TestSweepRetriesSlowOwnershipValidation(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.log")
	f.svc.PrepareOwners = func(context.Context, []string) core.OwnerBatchFunc {
		return func(context.Context, []string) []core.Ownership {
			return []core.Ownership{{Root: f.dir, Current: func(context.Context) (bool, time.Time, error) {
				return true, time.Now().Add(-32 * time.Millisecond), nil
			}}}
		}
	}
	if rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Hour)
	rows, err := f.svc.Sweep(false)
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || !strings.Contains(rows[0].Detail, "context deadline exceeded") {
		t.Fatalf("slow metadata validation did not retry: %+v %v", rows, err)
	}
	if read(t, path) != "keep.log" || len(f.items()) != 1 || len(f.burials()) != 0 {
		t.Fatal("slow metadata validation spent stale move authority")
	}
}

func TestGitVerificationBudget(t *testing.T) {
	for _, tc := range []struct{ elapsed, want time.Duration }{
		{0, 250 * time.Millisecond},
		{100 * time.Millisecond, 250 * time.Millisecond},
		{350 * time.Millisecond, 700 * time.Millisecond},
		{950 * time.Millisecond, 1900 * time.Millisecond},
		{15 * time.Second, 2 * time.Second},
	} {
		if got := core.GitVerificationBudget(tc.elapsed); got != tc.want {
			t.Errorf("Git verification budget for %s: got %s, want %s", tc.elapsed, got, tc.want)
		}
	}
}

func TestSweepSlowGitPreservesTracked(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git index publication and delayed queries")
	}
	for _, timing := range []string{"prefetch", "between-moves", "expired", "uncached"} {
		t.Run(timing, func(t *testing.T) {
			f, paths, git := slowGitFixture(t, "first.log", "keep.log")
			published := false
			publish := func() {
				if !published {
					git("add", "--", "keep.log")
					published = true
				}
			}
			f.svc.PrepareOwners = func(ctx context.Context, paths []string) core.OwnerBatchFunc {
				query := auto.PrepareOwners(ctx, paths)
				if timing == "uncached" {
					query = auto.OwnerBatch
				}
				return func(ctx context.Context, paths []string) []core.Ownership {
					owners := delayedOwners(ctx, query, paths)
					if timing == "prefetch" || timing == "uncached" {
						publish()
					}
					return owners
				}
			}
			firstMoved := false
			restore := core.SetBurialStep(f.svc, func(step string) error {
				if step == "moved" {
					firstMoved = true
				}
				if timing == "between-moves" && step == "moved" {
					publish()
				}
				if timing == "expired" && firstMoved && step == "git-checked" && !published {
					publish()
					time.Sleep(20 * time.Millisecond)
				}
				return nil
			})
			defer restore()
			for attempt := 0; attempt < 3; attempt++ {
				rows, err := f.svc.Sweep(false)
				if data, readErr := os.ReadFile(paths[1]); readErr != nil || string(data) != "keep.log" {
					t.Fatalf("slow Git moved newly tracked content: %+v %v", rows, readErr)
				}
				if err != nil || !published {
					t.Fatalf("slow Git publication fixture failed: %+v %v", rows, err)
				}
				items := f.items()
				if len(items) == 0 {
					if len(f.burials()) != 1 {
						t.Fatalf("slow Git publication lost sibling burial: %+v", rows)
					}
					return
				}
				// A final ownership check stalled beyond 10ms retains a retry.
				for _, item := range items {
					if item.NextAttempt.After(f.clock) {
						f.clock = item.NextAttempt
					}
				}
			}
			t.Fatalf("slow Git publication did not settle safely: %+v", f.items())
		})
	}
}
