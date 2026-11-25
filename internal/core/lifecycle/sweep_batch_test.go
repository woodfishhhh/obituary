package lifecycle_test

import (
	"context"
	"errors"
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
)

func dueBatch(t *testing.T, n int) (*fixture, []string) {
	t.Helper()
	f := newFixture(t)
	paths := make([]string, n)
	for i := range paths {
		paths[i] = f.file(fmt.Sprintf("item-%02d.txt", i))
	}
	if rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}); err != nil || core.Failed(rows) {
		t.Fatalf("add: %v %+v", err, rows)
	}
	f.clock = f.clock.Add(time.Hour)
	return f, paths
}

func TestSweepInvalidatedSnapshotReleasesLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git index publication")
	}
	// Check the lock boundary around a real Git refresh, independent of
	// scheduling time consumed before index publication or sibling moves.
	synctest.Test(t, func(t *testing.T) {
		f, paths, git := slowGitFixture(t, "first.log", "keep.log", "last.log")
		assertRefresh := invalidatedIndex(t, f, git, "keep.log", true)
		rows, err := f.svc.Sweep(false)
		assertRefresh()
		if err != nil || len(rows) != 3 || rows[0].Action != core.ActionBuried || rows[1].Action != core.ActionRefused || rows[2].Action != core.ActionBuried {
			t.Fatalf("refresh lost committed prefix or remaining siblings: %+v %v", rows, err)
		}
		if read(t, paths[1]) != "keep.log" || len(f.burials()) != 2 || len(f.items()) != 0 {
			t.Fatal("refresh moved protected content or lost ledger state")
		}
	})
}

func TestSweepInvalidatedSnapshotRefreshesOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git indexes published after consecutive queries")
	}
	// Index publication and lock boundaries use real Git; runner scheduling
	// must not consume the generation check's budget in this ordering test.
	synctest.Test(t, func(t *testing.T) {
		f, paths, git := slowGitFixture(t, "keep.log")
		f.file("seed.txt")
		git("add", "--", "seed.txt")
		index := filepath.Join(f.dir, ".git", "index")
		seed := read(t, index)
		git("add", "--", "keep.log")
		trackedIndex := read(t, index)
		if err := os.WriteFile(index, []byte(seed), 0600); err != nil {
			t.Fatal(err)
		}
		calls := 0
		f.svc.PrepareOwners = func(ctx context.Context, paths []string, roots map[string]string) core.OwnerBatchFunc {
			query := auto.PrepareOwners(ctx, paths, roots)
			return func(ctx context.Context, paths []string) []core.Ownership {
				calls++
				if unlocked := ledgerUnlocked(t, f); !unlocked {
					t.Fatalf("refresh query %d used wrong lock boundary: unlocked=%t", calls, unlocked)
				}
				owners := query(ctx, paths)
				if calls <= 2 {
					data := seed
					if calls == 2 {
						data = trackedIndex
					}
					if err := os.WriteFile(index+".next", []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(index+".next", index); err != nil {
						t.Fatal(err)
					}
				}
				return owners
			}
		}
		rows, err := f.svc.Sweep(false)
		if err != nil || calls != 2 || len(rows) != 1 || rows[0].Action != core.ActionRetry || read(t, paths[0]) != "keep.log" || len(f.burials()) != 0 {
			t.Fatalf("repeated invalidation skipped bounded retry or protection: calls=%d %+v %v", calls, rows, err)
		}
	})
}

func TestSweepSnapshotRefreshRechecksRevision(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git index invalidation and concurrent lifespan extension")
	}
	// Real Git invalidation must reach the unlocked revision change;
	// a scheduler stall must not turn this ordering check into a timeout.
	synctest.Test(t, func(t *testing.T) {
		f, paths, git := slowGitFixture(t, "keep.log")
		calls := 0
		f.svc.PrepareOwners = func(ctx context.Context, paths []string, roots map[string]string) core.OwnerBatchFunc {
			query := auto.PrepareOwners(ctx, paths, roots)
			return func(ctx context.Context, paths []string) []core.Ownership {
				calls++
				owners := query(ctx, paths)
				if calls == 1 {
					f.file("seed.txt")
					git("add", "--", "seed.txt")
				}
				if calls == 2 {
					if !ledgerUnlocked(t, f) {
						t.Fatal("refresh did not release ledger for revision change")
					}
					store := f.svc.Store.WithContext(ctx)
					if err := store.Update(func(st *core.State) error {
						st.Items[0].Rev++
						st.Items[0].ExpiresAt = f.clock.Add(time.Hour)
						return nil
					}); err != nil {
						t.Fatalf("refresh did not release ledger for revision change: %v", err)
					}
				}
				return owners
			}
		}
		rows, err := f.svc.Sweep(false)
		if err != nil || calls != 2 || len(rows) != 1 || rows[0].Action != core.ActionSkipped || read(t, paths[0]) != "keep.log" || len(f.burials()) != 0 || len(f.items()) != 1 {
			t.Fatalf("refreshed snapshot bypassed concurrent revision: %+v %v", rows, err)
		}
	})
}

func TestSweepSnapshotRefreshYieldsBeforeQuery(t *testing.T) {
	// The ownership callback controls invalidation; actual lock probes and
	// moves must reach both queries without consuming wall-clock budgets.
	synctest.Test(t, func(t *testing.T) {
		f := newFixture(t)
		path := f.file("scratch.log")
		calls := 0
		f.svc.PrepareOwners = func(context.Context, []string, map[string]string) core.OwnerBatchFunc {
			return func(context.Context, []string) []core.Ownership {
				calls++
				if !ledgerUnlocked(t, f) {
					t.Fatalf("snapshot refresh held store.lock: query=%d", calls)
				}
				valid := calls > 1
				return []core.Ownership{{Root: f.dir, Current: func(context.Context) (bool, time.Time, error) {
					return valid, time.Now(), nil
				}}}
			}
		}
		if rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
			t.Fatalf("add: %+v %v", rows, err)
		}
		f.clock = f.clock.Add(time.Hour)
		rows, err := f.svc.Sweep(false)
		if err != nil || calls != 2 || len(rows) != 1 || rows[0].Action != core.ActionBuried {
			t.Fatalf("snapshot refresh did not resume burial: calls=%d %+v %v", calls, rows, err)
		}
	})
}

func TestSweepBatchRecoversAllMoved(t *testing.T) {
	core.RequireFsync(t)
	for _, failure := range []string{"moved", "write", "unconfirmed"} {
		t.Run(failure, func(t *testing.T) {
			f, paths := dueBatch(t, 3)
			stop := errors.New("interrupted batch")
			moves := 0
			blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
			restoreStep := core.SetBurialStep(f.svc, func(step string) error {
				if step == "moved" {
					moves++
					if moves == 3 && failure == "moved" {
						return stop
					}
					if moves == 3 && failure == "write" {
						if err := os.Mkdir(blocked, 0700); err != nil {
							t.Fatal(err)
						}
					}
				}
				return nil
			})
			defer restoreStep()
			restoreSync := core.SetReplacementSync(func(file *os.File) error {
				if moves == 3 && failure == "unconfirmed" && strings.HasSuffix(file.Name(), "store.json.tmp") {
					return stop
				}
				return file.Sync()
			})
			defer restoreSync()
			rows, err := f.svc.Sweep(false)
			restoreStep()
			restoreSync()
			if failure == "write" {
				if err := os.Remove(blocked); err != nil {
					t.Fatal(err)
				}
			}
			if err == nil || moves != 3 || len(rows) != 3 {
				t.Fatalf("batch interruption missing: %d %+v %v", moves, rows, err)
			}
			for i, row := range rows {
				if exists(paths[i]) || row.Action != core.ActionRecoveryPending || row.Burial != nil || !exists(filepath.Dir(row.Grave)+".json") {
					t.Fatalf("batch lost uncommitted move evidence: %+v", rows)
				}
				if read(t, row.Grave) != filepath.Base(paths[i]) {
					t.Fatal("moved content changed")
				}
			}
			if failure != "unconfirmed" && len(f.burials()) != 0 {
				t.Fatal("failed batch partially committed")
			}
			restarted := *f.svc
			if out, err := restarted.Sweep(false); err != nil || len(out) != 3 {
				t.Fatalf("recover batch: %+v %v", out, err)
			}
			if out, err := restarted.Sweep(false); err != nil || len(out) != 0 {
				t.Fatalf("recovery repeated moves: %+v %v", out, err)
			}
			st, err := restarted.Store.ReadSnapshot()
			if err != nil || len(st.Items) != 0 || len(st.Burials) != 3 || st.Stats.Buried != 3 || len(st.PendingAudit) != 0 {
				t.Fatalf("recovery lost or duplicated batch: %+v %v", st, err)
			}
			for _, b := range st.Burials {
				if exists(filepath.Dir(b.Grave) + ".json") {
					t.Fatal("recovered manifest remained")
				}
			}
		})
	}
}

func TestSweepBatchRechecksEachRevision(t *testing.T) {
	for _, change := range []string{"extend", "cancel"} {
		t.Run(change, func(t *testing.T) {
			f, paths := dueBatch(t, 3)
			f.svc.FS = &prepareHookFS{testFS: f.fs, before: func() {
				var err error
				if change == "extend" {
					_, err = f.svc.Extend([]string{paths[1]}, time.Hour)
				} else {
					_, err = f.svc.Cancel([]string{paths[1]})
				}
				if err != nil {
					t.Fatal(err)
				}
			}}
			out, err := f.svc.Sweep(false)
			if err != nil || exists(paths[0]) || !exists(paths[1]) || exists(paths[2]) || len(f.burials()) != 2 {
				t.Fatalf("batch revision check did not isolate changed item: %+v %v", out, err)
			}
			if change == "extend" && (len(f.items()) != 1 || !f.items()[0].ExpiresAt.After(f.clock)) {
				t.Fatal("batch overwrote extended revision")
			}
		})
	}
}

func TestSweepBatchPinnedIdentity(t *testing.T) {
	f, paths := dueBatch(t, 3)
	backup := paths[1] + ".backup"
	f.fs.onRecycle = func(string) {
		f.fs.onRecycle = nil
		if err := os.Rename(paths[1], backup); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[1], []byte("replacement"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := f.svc.Sweep(false)
	data, readErr := os.ReadFile(paths[1])
	if err != nil || readErr != nil || exists(paths[0]) || exists(paths[2]) || !exists(backup) || string(data) != "replacement" || len(f.burials()) != 2 || len(f.items()) != 1 {
		t.Fatalf("batch moved a replacement or lost another item: %+v %v", out, err)
	}
}

func TestSweepBatchReleasesLedgerBetweenSlowMoves(t *testing.T) {
	f, _ := dueBatch(t, 3)
	moves := 0
	yielded := false
	t.Cleanup(core.SetBurialStep(f.svc, func(step string) error {
		if step == "committed" && moves == 1 {
			if !ledgerUnlocked(t, f) {
				t.Fatal("slow batch committed prefix while holding store.lock")
			}
			yielded = true
		}
		return nil
	}))
	f.fs.onRecycle = func(string) {
		moves++
		if moves == 1 {
			// Exhaust the batch's work budget; lock assertions do not wait.
			time.Sleep(120 * time.Millisecond)
		}
		if moves == 2 && !yielded {
			t.Error("slow batch kept ledger locked across its work budget")
		}
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if moves != 3 || !yielded {
		t.Fatalf("slow batch did not resume after yielding: moves=%d yielded=%t", moves, yielded)
	}
}

func TestSweepBatchSkipsAbsorbedLaterEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	f := newFixture(t)
	child := f.file("parent/child.txt")
	paths := []string{filepath.Dir(child)}
	for i := 0; i < 31; i++ {
		paths = append(paths, f.file(fmt.Sprintf("independent-%02d", i)))
	}
	paths = append(paths, child)
	if rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}); err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Hour)
	out, err := f.svc.Sweep(false)
	if err != nil || len(f.items()) != 0 || len(f.burials()) != 32 {
		t.Fatalf("parent batch lost contained entries: %+v %v", out, err)
	}
	for _, row := range out {
		if row.Action == core.ActionRetry || row.Action == core.ActionBlocked {
			t.Fatalf("later batch retried an already absorbed child: %+v", row)
		}
	}
}

func TestSweepBatchChangesAcrossBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	// Cross-batch revisions and real Git publication depend on move order,
	// not the runner's speed while it processes the first 32 entries.
	synctest.Test(t, func(t *testing.T) {
		f := newFixture(t)
		git := func(args ...string) {
			t.Helper()
			if out, err := exec.Command("git", append([]string{"-C", f.dir}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git: %s %v", out, err)
			}
		}
		git("init", "-q")
		paths := make([]string, 67)
		for i := range paths {
			paths[i] = f.file(fmt.Sprintf("item-%02d.log", i))
		}
		f.svc.Owner, f.svc.OwnerBatch = auto.Owner, auto.OwnerBatch
		changed, cachedGitAnswer := false, false
		f.svc.PrepareOwners = func(ctx context.Context, candidates []string, roots map[string]string) core.OwnerBatchFunc {
			query := auto.PrepareOwners(ctx, candidates, roots)
			return func(ctx context.Context, candidates []string) []core.Ownership {
				answers := query(ctx, candidates)
				for i, path := range candidates {
					if !changed && path == paths[35] && answers[i].Err == nil && !answers[i].Tracked && answers[i].Current != nil {
						cachedGitAnswer = true
					}
				}
				return answers
			}
		}
		if rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir}); err != nil || core.Failed(rows) {
			t.Fatalf("add: %+v %v", rows, err)
		}
		f.clock = f.clock.Add(time.Hour)
		moves, prepared := 0, 0
		restore := core.SetBurialStep(f.svc, func(step string) error {
			if step == "moved" {
				moves++
			}
			if step == "manifest" {
				prepared++
			}
			// The first 32 items have committed and the second batch has read
			// its ledger snapshot. Change later items before their final checks;
			// their Git answers were prefetched in the first query.
			if step == "manifest" && prepared == 33 && moves == 32 && !changed {
				if !cachedGitAnswer {
					t.Fatal("cross-batch fixture did not cache the later Git answer")
				}
				changed = true
				if _, err := f.svc.Extend([]string{paths[33]}, time.Hour); err != nil {
					t.Fatal(err)
				}
				if _, err := f.svc.Cancel([]string{paths[34]}); err != nil {
					t.Fatal(err)
				}
				git("add", "--", filepath.Base(paths[35]))
			}
			return nil
		})
		defer restore()
		rows, err := f.svc.Sweep(false)
		st, readErr := f.svc.Store.ReadSnapshot()
		if !changed {
			t.Fatal("cross-batch change fixture was not reached")
		}
		for _, path := range paths[33:36] {
			if data, readErr := os.ReadFile(path); readErr != nil || string(data) != filepath.Base(path) {
				t.Fatalf("cross-batch changes moved protected content: %s %+v %v", path, rows, readErr)
			}
		}
		if err != nil || readErr != nil || len(st.Burials) != 64 || st.Stats.Buried != 64 || len(st.Items) != 1 || st.Items[0].Path != paths[33] || !st.Items[0].ExpiresAt.After(f.clock) {
			t.Fatalf("cross-batch changes affected unrelated entries: %+v %+v %v %v", rows, st, err, readErr)
		}
		seen := make(map[string]bool)
		for _, b := range st.Burials {
			if seen[b.Path] || exists(b.Path) || read(t, b.Grave) != filepath.Base(b.Path) {
				t.Fatalf("cross-batch burial was lost or duplicated: %+v", b)
			}
			seen[b.Path] = true
		}
		for i, path := range paths {
			if (i < 33 || i >= 36) && !seen[path] {
				t.Fatalf("cross-batch changes skipped an unrelated item: %s", path)
			}
		}
	})
}

const slowStartup = 1200 * time.Millisecond

func delayGit(ctx context.Context) {
	timer := time.NewTimer(slowStartup)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

type snapshotRead struct {
	wait time.Duration
	err  error
}

// Start the deadline at the observed query boundary, independently of sweep setup.
func concurrentSnapshot(store core.Store) <-chan snapshotRead {
	started := make(chan struct{})
	done := make(chan snapshotRead, 1)
	go func() {
		start := time.Now()
		reader := store.WithDeadline(start.Add(time.Second))
		close(started)
		_, err := reader.ReadSnapshot()
		done <- snapshotRead{time.Since(start), err}
	}()
	<-started
	return done
}

func TestSlowGitConcurrentSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git publications and a one-second ledger reader")
	}
	for _, mode := range []string{"invalidated", "twice", "include", "sparse"} {
		t.Run(mode, func(t *testing.T) {
			// Real Git and ledger locks; only delay/deadline clocks are virtual.
			synctest.Test(t, func(t *testing.T) { testConcurrentSnapshot(t, mode) })
		})
	}
}

func testConcurrentSnapshot(t *testing.T, mode string) {
	f, paths, git := slowGitFixture(t, "keep.log")
	uncached := mode == "include" || mode == "sparse"
	configureSlowGit(f, git, mode)
	publication := 1
	if mode == "twice" {
		publication = 2
	}
	calls := 0
	var reader <-chan snapshotRead
	f.svc.PrepareOwners = func(ctx context.Context, paths []string, roots map[string]string) core.OwnerBatchFunc {
		query := auto.PrepareOwners(ctx, paths, roots)
		return func(ctx context.Context, paths []string) []core.Ownership {
			calls++
			if uncached && calls == 2 {
				reader = concurrentSnapshot(f.svc.Store)
			}
			delayGit(ctx)
			owners := query(ctx, paths)
			if calls == 1 && (owners[0].Current == nil) != uncached {
				t.Fatalf("unexpected cacheability: %+v", owners[0])
			}
			if calls < publication {
				git("add", "--", "seed")
			} else if calls == publication {
				git("add", "--", "keep.log")
			}
			if !uncached && calls == publication {
				current := owners[0].Current
				owners[0].Current = func(ctx context.Context) (bool, time.Time, error) {
					ok, at, err := current(ctx)
					if ok || err != nil {
						t.Fatalf("expected published index: %v %v", ok, err)
					}
					reader = concurrentSnapshot(f.svc.Store)
					return ok, at, err
				}
			}
			return owners
		}
	}
	rows, err := f.svc.Sweep(false)
	if err != nil || len(rows) != 1 || reader == nil || read(t, paths[0]) != "keep.log" || len(f.burials()) != 0 {
		t.Fatalf("protected file or read missing: %+v %v", rows, err)
	}
	checkSlowGitRead(t, f, mode, calls, rows, <-reader)
}

func configureSlowGit(f *fixture, git func(...string), mode string) {
	if mode == "twice" {
		f.file("seed")
	}
	if mode == "include" {
		git("config", "include.path", "optional.config")
	}
	if mode == "sparse" {
		f.file("omitted/tracked")
		git("add", "--", "omitted/tracked")
		git("commit", "-qm", "seed sparse index")
		git("sparse-checkout", "init", "--cone", "--sparse-index")
		git("sparse-checkout", "set", "kept")
	}
}

func checkSlowGitRead(t *testing.T, f *fixture, mode string, calls int, rows []core.Outcome, r snapshotRead) {
	t.Helper()
	uncached := mode == "include" || mode == "sparse"
	t.Logf("reader=%s err=%v action=%s queries=%d", r.wait, r.err, rows[0].Action, calls)
	want := core.ActionRefused
	if mode == "twice" {
		want = core.ActionRetry
		requireRetainedRetry(t, f, rows)
	}
	if rows[0].Action != want || calls != 2 || (uncached && !errors.Is(r.err, context.DeadlineExceeded)) || (!uncached && r.err != nil) {
		t.Fatal("unexpected read or final Git outcome")
	}
}
