package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestSweepCommitUsesHotTransaction(t *testing.T) {
	for _, action := range []string{core.ActionRetry, core.ActionRecycled, core.ActionVanished, core.ActionRefused} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t)
			it := f.add(f.file("tracked"), time.Hour)
			before, restore := unreadableSnapshot(t, f.svc.Store)
			out := core.Outcome{Item: it, Path: it.Path, Action: action, Detail: "retry reason"}
			if err := core.CommitSweepOutcome(f.svc, out); err != nil {
				t.Fatal("sweep outcome read cold evidence", err)
			}
			restore()
			after, err := f.svc.Store.ReadSnapshot()
			if err != nil || *after.Graveyard != *before.Graveyard {
				t.Fatal("sweep outcome changed graveyard", err)
			}
			if action == core.ActionRetry {
				if len(after.Items) != 1 || after.Items[0].Rev != it.Rev+1 || after.Items[0].Attempts != 1 || after.Items[0].LastError != out.Detail || !after.Items[0].NextAttempt.After(f.clock) {
					t.Fatal("sweep retry lost revision or backoff", after.Items)
				}
			} else if len(after.Items) != 0 {
				t.Fatal("sweep outcome retained completed item", after.Items)
			}
		})
	}
}

func BenchmarkSweepNoDue(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			svc, _ := benchmarkGraveyard(b, n)
			b.ResetTimer()
			for b.Loop() {
				if _, err := svc.Sweep(false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Only Sweep is timed. Fixtures use actual Windows file IDs and the normal
// durable store, with both the objects and the graveyard on the TEMP volume.
func BenchmarkSweepDue(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				dir := b.TempDir()
				fs := &testFS{}
				svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "home")}, FS: fs}
				items := make([]core.Item, n)
				for j := range items {
					path := filepath.Join(dir, fmt.Sprintf("payload-%04d", j))
					if err := os.WriteFile(path, []byte("payload"), 0600); err != nil {
						b.Fatal(err)
					}
					id, err := fs.Identify(path)
					if err != nil {
						b.Fatal(err)
					}
					items[j] = core.Item{ID: int64(j + 1), Rev: 1, Path: path, Identity: &id, ExpiresAt: time.Now().Add(-time.Hour)}
				}
				if err := svc.Store.Update(func(st *core.State) error { st.Items = items; st.NextID = int64(n + 1); return nil }); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				out, err := svc.Sweep(false)
				b.StopTimer()
				if err != nil || len(out) != n {
					b.Fatalf("sweep %d: %d outcomes, %v", n, len(out), err)
				}
				st, err := svc.Store.ReadSnapshot()
				if err != nil || len(st.Items) != 0 || len(st.Burials) != n || st.Stats.Buried != int64(n) {
					b.Fatalf("incomplete sweep: %v", err)
				}
			}
		})
	}
}

func TestSweepRechecksGuardAfterManifest(t *testing.T) {
	f := newFixture(t)
	first, second := f.file("allowed.txt"), f.file("protected.txt")
	f.add(first, time.Hour)
	f.add(second, time.Hour)
	f.clock = f.clock.Add(time.Hour)
	seen := 0
	restore := core.SetAfterManifest(f.svc, func() {
		seen++
		if seen == 2 {
			f.svc.Guard.Keep[0] = second
		}
	})
	defer restore()
	out, err := f.svc.Sweep(false)
	if err != nil || len(out) != 2 || exists(first) || !exists(second) || len(f.burials()) != 1 {
		t.Fatalf("final guard change must spare only protected item: %+v %v", out, err)
	}
}

func TestSweepFinishesAfterCancellation(t *testing.T) {
	for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("expired.log")
			f.add(path, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			f.fs.onRecycle = func(string) { cancel() }
			svc := f.svc.WithContext(ctx)
			svc.Mode = mode
			out, err := svc.Sweep(false)
			if err != nil || len(out) != 1 || len(f.items()) != 0 || exists(path) {
				t.Fatalf("sweep canceled after filesystem move: %+v %v", out, err)
			}
			if mode == core.ModeGraveyard && len(f.burials()) != 1 {
				t.Fatal("sweep move missing burial")
			}
		})
	}
}

func TestGuardIOFailureRetriesDueItem(t *testing.T) {
	for _, kind := range []string{"canonical", "repository-walk"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("work/keep.txt")
			if kind == "repository-walk" {
				path = filepath.Join(f.dir, "work")
			}
			old := f.add(path, time.Hour)
			if kind == "canonical" {
				f.svc.Guard.Canonical = func(string) (string, error) { return "", errors.New("temporarily unavailable") }
			} else {
				f.svc.Guard.WalkLimit = 1
				if err := f.svc.Guard.Check(path, true); err != nil {
					t.Fatalf("force must still bypass an incomplete repository check: %v", err)
				}
			}
			out := f.sweepAt(time.Hour)
			if len(out) != 1 || out[0].Action != core.ActionRetry {
				t.Errorf("I/O failure must retry: %+v", out)
			}
			items := f.items()
			if len(items) != 1 || items[0].ID != old.ID || items[0].Attempts != 1 || !items[0].NextAttempt.After(f.clock) {
				t.Errorf("retry record lost: %+v", items)
			}
			if _, err := os.Stat(path); err != nil || len(f.fs.recycled) != 0 {
				t.Fatalf("failed guard moved file: %v, %v", err, f.fs.recycled)
			}
		})
	}
}

func TestMoveIdentitySwapRemainsRetryable(t *testing.T) {
	f := newFixture(t)
	path := f.file("editor.txt")
	item := f.add(path, time.Hour)
	backup := path + ".bak"
	f.fs.onRecycle = func(string) {
		f.fs.onRecycle = nil
		if err := os.Rename(path, backup); err != nil {
			t.Fatal(err)
		}
		f.file("editor.txt")
	}
	out := f.sweepAt(time.Hour)
	items := f.items()
	if len(out) != 1 || out[0].Action != core.ActionRetry || len(items) != 1 || items[0].ID != item.ID || items[0].Attempts != 1 || !exists(path) || !exists(backup) {
		t.Fatalf("identity swap discarded the original's retry: %+v, %+v", out, items)
	}
	f.sweepAt(time.Minute)
	if !exists(path) || exists(backup) || len(f.burials()) != 1 {
		t.Fatal("retry must follow the original to its backup name")
	}
}

func TestBlockedItemGetsRetryBackoff(t *testing.T) {
	f := newFixture(t)
	item := f.add(f.file("keep.txt"), time.Hour)
	f.clock = f.clock.Add(time.Hour)
	fs := &scanFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, item.Path)}
	f.svc.FS = fs
	out, _, err := f.svc.Tick()
	if err != nil || len(out) != 1 || out[0].Action != core.ActionBlocked {
		t.Fatalf("block: %+v, %v", out, err)
	}
	items := f.items()
	if len(items) != 1 || items[0].Attempts != 1 || !items[0].NextAttempt.Equal(f.clock.Add(time.Minute)) || items[0].LastError == "" {
		t.Fatalf("blocked entry has no retry backoff: %+v", items)
	}
	fs.failed = false
	if _, _, err := f.svc.Tick(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, f.items()) {
		t.Fatal("retry before NextAttempt changed backoff")
	}
}

func TestExpiredItemIsRecycledAndForgotten(t *testing.T) {
	f := newFixture(t)
	log := f.file("debug.log")
	f.add(log, time.Hour)

	if out := f.sweepAt(59 * time.Minute); len(out) != 0 || !exists(log) {
		t.Fatalf("swept before expiry: %+v", out)
	}
	out := f.sweepAt(time.Minute)
	if len(out) != 1 || out[0].Action != core.ActionBuried || exists(log) {
		t.Fatalf("want recycled at expiry, got %+v (exists=%v)", out, exists(log))
	}
	if items := f.items(); len(items) != 0 {
		t.Fatalf("item still tracked: %+v", items)
	}
	events, _ := f.svc.Store.History(10)
	if last := events[len(events)-1]; last.Action != core.ActionBuried || last.Path != log {
		t.Fatalf("audit log missing recycle: %+v", events)
	}
}

func TestRenamedItemIsFollowedToItsNewPath(t *testing.T) {
	f := newFixture(t)
	old := f.file("a/shot.png")
	f.add(old, time.Hour)
	moved := filepath.Join(f.dir, "b", "renamed.png")
	if err := os.MkdirAll(filepath.Dir(moved), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(old, moved); err != nil {
		t.Fatal(err)
	}

	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionBuried || !samePath(out[0].Path, moved) || exists(moved) {
		t.Fatalf("want renamed file recycled at %s, got %+v", moved, out)
	}
}

func TestDifferentFileAtTheSamePathIsLeftAlone(t *testing.T) {
	f := newFixture(t)
	path := f.file("report.txt")
	f.add(path, time.Hour)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	f.file("report.txt") // a new file reuses the name

	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionReplaced || !exists(path) {
		t.Fatalf("want replacement kept, got %+v (exists=%v)", out, exists(path))
	}
	if len(f.items()) != 0 {
		t.Fatal("replaced item should no longer be tracked")
	}
}

func TestVanishedItemIsDroppedWithoutRecycling(t *testing.T) {
	f := newFixture(t)
	path := f.file("tmp.txt")
	f.add(path, time.Hour)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionVanished || len(f.fs.recycled) != 0 {
		t.Fatalf("want vanished, got %+v recycled=%v", out, f.fs.recycled)
	}
}

func TestFailedRecycleIsRetriedWithBackoff(t *testing.T) {
	f := newFixture(t)
	path := f.file("locked.db")
	f.add(path, time.Hour)
	f.fs.fail = errors.New("in use")

	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("want retry, got %+v", out)
	}
	item := f.items()[0]
	if item.Attempts != 1 || item.LastError != "in use" || !item.NextAttempt.Equal(f.clock.Add(time.Minute)) {
		t.Fatalf("retry not recorded: %+v", item)
	}
	if out := f.sweepAt(30 * time.Second); len(out) != 0 {
		t.Fatalf("retried before backoff elapsed: %+v", out)
	}
	f.fs.fail = nil
	if out := f.sweepAt(30 * time.Second); len(out) != 1 || out[0].Action != core.ActionBuried || exists(path) {
		t.Fatalf("want recycled on retry, got %+v", out)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	f := newFixture(t)
	path := f.file("a.log")
	f.add(path, time.Hour)
	f.clock = f.clock.Add(time.Hour)

	out, err := f.svc.Sweep(true)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("dry run report %+v %v", out, err)
	}
	if !exists(path) || len(f.items()) != 1 {
		t.Fatal("dry run modified something")
	}
}

func TestFolderThatBecameARepositoryIsNotRecycled(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Join(f.dir, "scratch")
	f.file("scratch/notes.txt")
	f.add(dir, time.Hour)
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRefused || !exists(dir) {
		t.Fatalf("want refused and kept, got %+v", out)
	}
}

func TestCancelDuringSweepSparesLaterItems(t *testing.T) {
	f := newFixture(t)
	a, b := f.file("a.txt"), f.file("b.txt")
	f.add(a, time.Hour)
	itemB := f.add(b, time.Hour+time.Second)
	f.svc.FS = &prepareHookFS{testFS: f.fs, before: func() {
		if _, err := f.svc.Cancel([]string{"#" + strconv.FormatInt(itemB.ID, 10)}); err != nil {
			t.Error(err)
		}
	}}

	f.sweepAt(2 * time.Hour)
	if exists(a) || !exists(b) {
		t.Fatalf("want a recycled and b spared (a=%v b=%v)", exists(a), exists(b))
	}
}

func TestExtendDuringAFailedRecycleKeepsTheNewExpiry(t *testing.T) {
	f := newFixture(t)
	f.svc.Mode = core.ModeRecycle
	path := f.file("a.txt")
	f.add(path, time.Hour)
	f.fs.fail = errors.New("in use")
	f.fs.onRecycle = func(string) {
		if _, err := f.svc.Extend([]string{path}, 24*time.Hour); err != nil {
			t.Error(err)
		}
	}

	f.sweepAt(time.Hour)
	item := f.items()[0]
	if item.Attempts != 0 || item.LastError != "" || !item.ExpiresAt.Equal(f.clock.Add(24*time.Hour)) {
		t.Fatalf("the extension must win over the stale failure, got %+v", item)
	}
}

func TestRetryBackoffCountsFromTheFailure(t *testing.T) {
	f := newFixture(t)
	path := f.file("slow.bin")
	f.add(path, time.Hour)
	f.fs.fail = errors.New("in use")
	f.fs.onRecycle = func(string) { f.clock = f.clock.Add(5 * time.Minute) } // a slow failure

	f.sweepAt(time.Hour)
	if item := f.items()[0]; !item.NextAttempt.Equal(f.clock.Add(time.Minute)) {
		t.Fatalf("want next attempt 1m after the failure (%v), got %v", f.clock, item.NextAttempt)
	}
}

func TestUnknownLookupErrorKeepsTheItem(t *testing.T) {
	f := newFixture(t)
	path := f.file("a.txt")
	f.add(path, time.Hour)
	moved := filepath.Join(f.dir, "moved.txt")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	f.fs.locateErr = errors.New("access denied")

	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRetry || len(f.items()) != 1 || !exists(moved) {
		t.Fatalf("an unknown lookup failure must retry, not forget: %+v", out)
	}
}

func TestRecordWithoutIdentityIsNeverRecycled(t *testing.T) {
	f := newFixture(t)
	path := f.file("a.txt")
	f.add(path, time.Hour)
	if err := f.svc.Store.Update(func(st *core.State) error { st.Items[0].Identity = nil; return nil }); err != nil {
		t.Fatal(err)
	}

	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRefused || !exists(path) {
		t.Fatalf("want refused and kept, got %+v", out)
	}
}

func TestTickKeepsBurialsOnBlockedRoot(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	f.clock = b.PurgeAt
	f.svc.FS = &scanFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, b.Path)}
	out, purged, err := f.svc.Tick()
	if err != nil || len(out) != 1 || out[0].Action != core.ActionBlocked || len(purged) != 0 || len(f.burials()) != 1 || !exists(b.Grave) {
		t.Fatalf("Tick purged a blocked root later in the same round: %+v, %+v, %v", out, purged, err)
	}
	// A new round may retry once enumeration is possible and backoff elapsed.
	f.clock = f.clock.Add(time.Minute)
	_, purged, err = f.svc.Tick()
	if err != nil || len(purged) != 1 || purged[0].Action != core.ActionPurged {
		t.Fatalf("blocked burial never retried: %+v, %v", purged, err)
	}
}

func TestRecycleCommitPreservesConcurrentExtension(t *testing.T) {
	for _, operation := range []string{"sweep", "bury"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			f.svc.Mode = core.ModeRecycle
			path := f.file("racing.txt")
			item := f.add(path, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			var changed core.Item
			f.fs.onRecycle = func(string) {
				rows, err := f.svc.Extend([]string{fmt.Sprint(item.ID)}, 30*24*time.Hour)
				if err != nil || core.Failed(rows) {
					t.Fatalf("extension: %+v, %v", rows, err)
				}
				changed = *rows[0].Item
			}
			if operation == "sweep" {
				_, _ = f.svc.Sweep(false)
			} else {
				rows, err := f.svc.Bury([]string{path}, false, "")
				if err != nil || core.GraveFailed(rows) || rows[0].Action != core.ActionRecycled {
					t.Fatalf("receipt must report the completed Shell operation: %+v, %v", rows, err)
				}
			}
			if items := f.items(); len(items) != 1 || !reflect.DeepEqual(items[0], changed) {
				t.Fatalf("Shell completion erased the newer revision: %+v", items)
			}
		})
	}
}
