package lifecycle_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/watch"
)

type watchAfterMoveFS struct {
	core.FS
	after func(string, string)
}

func (fs watchAfterMoveFS) MovePinned(path string, id core.Identity, to string) error {
	if err := fs.FS.MovePinned(path, id, to); err != nil {
		return err
	}
	fs.after(path, to)
	return nil
}

func TestWatchAriaSidecarArrivesDuringMove(t *testing.T) {
	for _, stage := range []string{"observation", "move"} {
		t.Run(stage, func(t *testing.T) {
			f, path, _ := watchFixture(t, "file.zip", core.WatchFirstSeen)
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			publish := func() { f.file("downloads/file.zip.aria2") }
			if stage == "observation" {
				f.svc.FS = &watchObserveCallbackFS{FS: f.svc.FS, onHold: publish}
			} else {
				f.svc.FS = watchAfterMoveFS{FS: f.svc.FS, after: func(_, _ string) { publish() }}
			}
			out := f.sweepAt(2 * time.Hour)
			if !exists(path) || len(f.burials()) != 0 || len(out) != 1 || out[0].Action != core.ActionRetry {
				t.Fatalf("late original aria2 sidecar did not retain file: %+v", out)
			}
			if stage == "observation" && len(f.fs.recycled) != 0 {
				t.Fatal("late sidecar was not checked before movement")
			}
		})
	}
}

func TestWatchPostMoveChangeRetries(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("post-move watch validation requires Windows writer exclusion; POSIX retains the source before moving")
	}
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
	f.svc.FS = watchAfterMoveFS{FS: f.svc.FS, after: func(_, to string) {
		if err := os.WriteFile(filepath.Join(to, "file.zip"), []byte("new content"), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	out := approveWatchDirectoryAt(t, f, 2*time.Hour)
	if !exists(path) || len(out) != 1 || out[0].Action != core.ActionRetry || read(t, path) != "new content" {
		t.Fatalf("post-move change did not return intact: %+v", out)
	}
}

func TestWatchRollbackCollisionRetainsEvidence(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("watch rollback requires Windows writer exclusion; POSIX retains the source before moving")
	}
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	var grave string
	f.svc.FS = watchAfterMoveFS{FS: f.svc.FS, after: func(source, to string) {
		grave = to
		if err := os.WriteFile(filepath.Join(to, "file.zip"), []byte("fresh retained"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(source, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	out := approveWatchDirectoryAt(t, f, 2*time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRetry || !strings.Contains(out[0].Detail, "manual attention") {
		t.Fatalf("rollback collision did not request manual attention: %+v", out)
	}
	if read(t, path) != "replacement" || read(t, filepath.Join(grave, "file.zip")) != "fresh retained" {
		t.Fatal("rollback overwrote or lost content")
	}
	assertWatchQuarantine(t, f, grave)
}

func assertWatchQuarantine(t *testing.T, f *fixture, grave string) {
	t.Helper()
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || !strings.Contains(pending[0].Error, "manual attention") {
		t.Fatalf("doctor evidence missing: %+v %v", pending, err)
	}
	events, err := f.svc.Store.History(20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		found = found || e.Action == core.ActionRetry && strings.Contains(e.Detail, "manual attention")
	}
	if !found || len(f.items()) != 1 || f.items()[0].Attempts == 0 {
		t.Fatalf("rollback retry/audit missing: %+v", events)
	}
	f.sweepAt(24 * time.Hour)
	if len(f.burials()) != 0 || !exists(grave) {
		t.Fatal("recovery entombed an unverified directory")
	}
}

func TestWatchPostMoveObservationFailureRollsBack(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("post-move watch observation requires Windows writer exclusion; POSIX retains the source before moving")
	}
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.svc.FS = watchAfterMoveFS{FS: f.svc.FS, after: func(_, to string) {
		// A newly published partial download makes the post-move tree unreadable
		// to the stability observer, which must still restore the whole unit.
		if err := os.WriteFile(filepath.Join(to, "new.part"), []byte("partial"), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	out := approveWatchDirectoryAt(t, f, 2*time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRetry || !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("failed post-move observation did not roll back: %+v", out)
	}
	if read(t, filepath.Join(filepath.Dir(path), "new.part")) != "partial" {
		t.Fatal("rollback lost newly arrived child")
	}
}
