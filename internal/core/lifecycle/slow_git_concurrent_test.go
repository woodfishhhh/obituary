package lifecycle_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

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
	f.svc.PrepareOwners = func(ctx context.Context, paths []string) core.OwnerBatchFunc {
		query := auto.PrepareOwners(ctx, paths)
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

func TestSlowGitBudgetIsolatedByProject(t *testing.T) {
	if testing.Short() {
		t.Skip("two real repositories and delayed Git startup")
	}
	synctest.Test(t, testSlowGitBudgetByProject)
}

func testSlowGitBudgetByProject(t *testing.T) {
	f, slow, git := slowGitFixture(t, "slow.log")
	git("add", "--", "slow.log")
	fast := f.file("fast/keep.log")
	root := filepath.Dir(fast)
	git("-C", root, "init", "-q")
	git("-C", root, "config", "include.path", "optional.config")
	rows, err := f.svc.Add([]string{fast}, core.AddOptions{Project: root, Source: "rule", ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	git("-C", root, "add", "--", "keep.log")
	f.clock = f.clock.Add(time.Hour)
	var reader <-chan snapshotRead
	var budget time.Duration
	f.svc.PrepareOwners = func(ctx context.Context, paths []string) core.OwnerBatchFunc {
		query := auto.PrepareOwners(ctx, paths)
		return func(ctx context.Context, paths []string) []core.Ownership {
			if ledgerUnlocked(t, f) {
				if slices.Contains(paths, slow[0]) {
					delayGit(ctx)
				}
			} else if slices.Contains(paths, fast) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("missing final Git deadline")
				}
				budget = time.Until(deadline)
				reader = concurrentSnapshot(f.svc.Store)
				delayGit(ctx)
			}
			return query(ctx, paths)
		}
	}
	out, err := f.svc.Sweep(false)
	if err != nil || len(out) != 2 || reader == nil || len(f.burials()) != 0 || read(t, fast) != "fast/keep.log" || read(t, slow[0]) != "slow.log" {
		t.Fatalf("protected paths or reader missing: %+v %v", out, err)
	}
	r := <-reader
	t.Logf("fast budget=%s reader=%s err=%v actions=%s,%s", budget, r.wait, r.err, out[0].Action, out[1].Action)
	if budget != 250*time.Millisecond || r.err != nil || out[0].Action != core.ActionRefused || out[1].Action != core.ActionRetry {
		t.Fatal("slow repository inflated another project's final budget")
	}
}
