//go:build windows

package lifecycle_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/watch"
	"golang.org/x/sys/windows"
)

func TestWatchGitAddDuringTreeObservationStays(t *testing.T) {
	if testing.Short() {
		t.Skip("stages a file in a real Git repository")
	}
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.svc.FS = &watchObserveCallbackFS{FS: f.svc.FS, onHold: func() {
		out, err := exec.Command("git", "-C", f.dir, "add", "--", "downloads/tree/file.zip").CombinedOutput()
		if err != nil {
			t.Fatalf("git add: %s %v", out, err)
		}
	}}
	out := approveWatchDirectoryAt(t, f, 2*time.Hour)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("Git add during tree observation was buried: %+v", out)
	}
}

func TestWatchObservationsFinishBeforeFinalOwnerCheck(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/tree/file.zip")
	_, err := f.svc.SetWatch(filepath.Join(f.dir, "downloads"), "1h", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc) // every policy starts as a proposal
	var p core.WatchPolicy
	if ws, err := f.svc.Watches(); err != nil || len(ws) != 1 {
		t.Fatalf("approved policy: %+v %v", ws, err)
	} else {
		p = ws[0]
	}
	if err := f.svc.RegisterWatchChild(p, filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	observed, checked := false, false
	f.svc.FS = &watchObserveCallbackFS{FS: f.svc.FS, onHold: func() { observed = true }}
	f.svc.Owner = func(path string) (string, bool, error) {
		checked = observed
		return auto.Owner(path)
	}
	f.fs.onRecycle = func(string) {
		if !checked {
			t.Fatal("tree observation followed final ownership check")
		}
	}
	out := approveWatchDirectoryAt(t, f, 2*time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("verified directory did not move: %+v", out)
	}
}

func TestWatchGitFreshnessIncludesFinalCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, _ := watchFixture(t, "file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.svc.PrepareOwners = func(ctx context.Context, paths []string) core.OwnerBatchFunc {
		query := auto.PrepareOwners(ctx, paths)
		return func(ctx context.Context, paths []string) []core.Ownership {
			owners := query(ctx, paths)
			for i := range owners {
				current := owners[i].Current
				if current == nil {
					t.Fatal("real Git index snapshot unavailable")
				}
				owners[i].Current = func(ctx context.Context) (bool, time.Time, error) {
					ok, _, err := current(ctx)
					time.Sleep(15 * time.Millisecond)
					return ok, time.Now(), err
				}
			}
			return owners
		}
	}
	out := f.sweepAt(2 * time.Hour)
	if !exists(path) || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("slow final check reused fresh reply timestamp: %+v", out)
	}
}

func TestWatchPausedDownloadNamesSurvive(t *testing.T) {
	if testing.Short() {
		t.Skip("download policy evidence uses real Git repositories")
	}
	for _, name := range []string{"file.td", "file.TD.CFG", "file.xltd", "file.zip.!qB", "file.!UT", "file.!bt", "file.bc!", "file.opdownload", "file.crswap", "DwnlData/user/file.zip", "file.zip", "file.part", "file.download/payload"} {
		t.Run(name, func(t *testing.T) {
			f, path, _ := watchFixture(t, name, core.WatchFirstSeen)
			if name == "file.zip" {
				f.file("downloads/file.zip.aria2")
			}
			_ = watch.Reconcile(f.svc)
			out := f.sweepAt(2 * time.Hour)
			if !exists(path) || len(f.burials()) != 0 || len(f.items()) != 0 {
				t.Fatalf("paused download was registered or buried: %+v", out)
			}
		})
	}
}

func TestWatchFreshDescendantAfterFinalSnapshotStays(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.fs.onRecycle = func(p string) {
		if p != filepath.Dir(path) {
			return
		}
		f.fs.onRecycle = nil
		if err := os.WriteFile(path, []byte("fresh after release"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	out := approveWatchDirectoryAt(t, f, 2*time.Hour)
	if !exists(path) || len(f.burials()) != 0 || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("fresh descendant after snapshot was buried: %+v", out)
	}
	if got := read(t, path); got != "fresh after release" {
		t.Fatalf("rollback lost fresh content: %q", got)
	}
}

func TestWatchModifiedDirectoryUsesDescendantWriteAge(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchModified)
	if err := os.Chtimes(filepath.Dir(path), f.clock, f.clock); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	if err := os.Chtimes(path, f.clock, f.clock); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	want := f.clock.Add(time.Hour)
	out := f.sweepAt(11 * time.Minute)
	if !exists(path) || len(f.items()) != 1 || !f.items()[0].ExpiresAt.Equal(want) {
		t.Fatalf("descendant write did not receive a full modified lifespan: %+v", out)
	}
}

func TestWatchTreeSnapshotUsesRelativeNamesAndNewestWrite(t *testing.T) {
	f := newFixture(t)
	path := f.file("tree/file.zip")
	dir := filepath.Dir(path)
	if err := os.Chtimes(dir, f.clock, f.clock); err != nil {
		t.Fatal(err)
	}
	latest := f.clock.Add(time.Hour)
	if err := os.Chtimes(path, latest, latest); err != nil {
		t.Fatal(err)
	}
	id, err := f.svc.FS.Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, release, err := core.ObserveWatchTree(f.svc, dir, id, true, true)
	if err != nil {
		t.Fatal(err)
	}
	release()
	moved := dir + "-moved"
	if err := f.svc.FS.MovePinned(dir, id, moved); err != nil {
		t.Fatal(err)
	}
	after, release, err := core.ObserveWatchTree(f.svc, moved, id, true, true)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if before != after || !before.Modified.Equal(latest) {
		t.Fatalf("tree snapshot lost relocation or latest write: %+v %+v", before, after)
	}
}

func TestWatchNewItemWaitsForStability(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/old.txt")
	p, err := f.svc.SetWatch(filepath.Dir(path), "1s", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	if err := f.svc.RegisterWatchChild(p, path); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(time.Second)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("new watch item bypassed stability: %+v", out)
	}
	out = f.sweepAt(core.WatchStableWindow + time.Minute)
	if len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("stable item did not expire: %+v", out)
	}
}

func TestWatchFinalWriteCheck(t *testing.T) {
	for _, age := range []string{core.WatchFirstSeen, core.WatchModified} {
		t.Run(age, func(t *testing.T) {
			f, path, _ := watchFixture(t, "file.zip", age)
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			f.clock = f.clock.Add(2 * time.Hour)
			f.svc.FS = &watchObserveCallbackFS{FS: f.svc.FS, onHold: func() {
				if err := os.WriteFile(path, []byte("written after scan"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, f.clock, f.clock); err != nil {
					t.Fatal(err)
				}
			}}
			out := f.sweepAt(0)
			if !exists(path) || len(f.burials()) != 0 || len(f.fs.recycled) != 0 {
				t.Fatalf("final write check missed activity: %+v", out)
			}
			it := f.items()[0]
			if age == core.WatchModified && it.ExpiresAt.Before(f.clock.Add(time.Hour)) {
				t.Fatal("final modified deadline was not refreshed")
			}
		})
	}
}

func TestWatchOpenWriterWithoutTemporaryName(t *testing.T) {
	f, path, _ := watchFixture(t, "file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	out := f.sweepAt(2 * time.Hour)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("open writer moved into grave: %+v", out)
	}
}

type watchMetadataErrorFS struct{ core.FS }

func (fs watchMetadataErrorFS) ObserveWatch(string, bool) (core.WatchSnapshot, func(), error) {
	return core.WatchSnapshot{}, nil, os.ErrPermission
}

func TestWatchUnreadableMetadataRetries(t *testing.T) {
	f, path, _ := watchFixture(t, "file.txt", core.WatchModified)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.svc.FS = watchMetadataErrorFS{f.svc.FS}
	if err := watch.Reconcile(f.svc); err == nil {
		t.Fatal("metadata scan error hidden")
	}
	out := f.sweepAt(2 * time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRetry || !exists(path) || len(f.items()) != 1 {
		t.Fatalf("unreadable metadata did not retry: %+v", out)
	}
}

func TestWatchModifiedPreservesManualExtensionAndAgeBasis(t *testing.T) {
	f, path, _ := watchFixture(t, "file.txt", core.WatchModified)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Extend([]string{path}, 24*time.Hour)
	if err != nil || core.Failed(rows) {
		t.Fatalf("extend: %+v %v", rows, err)
	}
	want := rows[0].Item.ExpiresAt
	if _, err := f.svc.SetWatch(filepath.Dir(path), "1s", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ApproveWatch(st.Proposals[0]); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(time.Hour)
	if err := os.Chtimes(path, f.clock, f.clock); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	it := f.items()[0]
	if !it.ExpiresAt.Equal(want) || it.Watch.Age != core.WatchModified || it.Watch.Lifespan != "1h" || !it.Watch.Automatic.Equal(f.clock.Add(time.Hour)) {
		t.Fatalf("manual extension or original age changed: %+v %+v", it, it.Watch)
	}
}

func TestWatchDownloadCompletionStartsObservation(t *testing.T) {
	f, path, _ := watchFixture(t, "file.crdownload", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	done := filepath.Join(filepath.Dir(path), "file.zip")
	if err := os.Rename(path, done); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	it := f.items()
	if len(it) != 1 || !it[0].CreatedAt.Equal(f.clock) || !it[0].ExpiresAt.Equal(f.clock.Add(time.Hour)) {
		t.Fatalf("completion retained temporary age: %+v", it)
	}
}

func TestWatchDirectoryContentActivity(t *testing.T) {
	f, path, _ := watchFixture(t, "tree/nested.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	if err := os.WriteFile(path, []byte("active child"), 0600); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(0)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("directory hid active descendant: %+v", out)
	}
	proposal := expiredWatchProposal(t, f)
	out = f.sweepAt(core.WatchStableWindow + time.Minute)
	if len(out) != 0 || core.ProposalFingerprint(expiredWatchProposal(t, f)) != core.ProposalFingerprint(proposal) || !exists(path) {
		t.Fatalf("waiting directory review changed or moved content: %+v", out)
	}
	out = approveWatchDirectoryAt(t, f, 0)
	rows, err := f.svc.Resurrect([]string{fmt.Sprint(out[0].Item.ID)})
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("restore: %+v %v", rows, err)
	}
}

func TestWatchModifiedFinalDeadlineAfterStability(t *testing.T) {
	f, path, _ := watchFixture(t, "file.txt", core.WatchModified)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	modified := f.clock.Add(-10 * time.Minute)
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(0)
	out := f.sweepAt(core.WatchStableWindow + time.Minute)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("modified final deadline lost after stability: %+v", out)
	}
	if !f.items()[0].ExpiresAt.Equal(modified.Add(time.Hour)) {
		t.Fatalf("final deadline not persisted: %+v", f.items())
	}
}

func TestWatchConfiguredStableWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, _ := watchFixture(t, "file.txt", core.WatchFirstSeen, "1s")
	f.svc.WatchStableWindow = 25 * time.Minute
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(11 * time.Minute)
	if !exists(path) || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("configured stable window ignored: %+v", out)
	}
	out = f.sweepAt(15 * time.Minute)
	if exists(path) || len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("configured stable window never completed: %+v", out)
	}
}

func TestWatchDryRunRespectsStability(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/file.txt")
	p, err := f.svc.SetWatch(filepath.Dir(path), "1s", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	if err := f.svc.RegisterWatchChild(p, path); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(time.Second)
	out, err := f.svc.Sweep(true)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("dry run ignored stability: %+v %v", out, err)
	}
}
