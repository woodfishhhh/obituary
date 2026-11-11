//go:build windows

package core_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/watch"
	"golang.org/x/sys/windows"
)

func TestWatchRegistrationProtectsSparedDescendant(t *testing.T) {
	if testing.Short() {
		t.Skip("watch registration uses a real Git repository")
	}
	f := newFixture(t)
	testenv.InitRepo(t, f.dir)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/bundle/keep.txt")
	f.add(path, time.Hour)
	if rows, err := f.svc.Cancel([]string{path}); err != nil || core.Failed(rows) {
		t.Fatalf("spare: %+v %v", rows, err)
	}
	p, err := f.svc.SetWatch(filepath.Dir(filepath.Dir(path)), "2s", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	err = f.svc.RegisterWatchChild(p, filepath.Dir(path))
	if err != nil || len(f.items()) != 1 || !exists(path) {
		t.Fatalf("watch failed to date the directory containing a kept object: %v", err)
	}
	f.sweepAt(time.Minute)
	pending := expiredWatchProposal(t, f)
	if len(pending.WatchBurial.Kept) != 1 || pending.WatchBurial.Kept[0] != path {
		t.Fatalf("watch proposal omitted pre-policy keep decision: %+v", pending)
	}
}

type watchRetryFS struct {
	core.FS
	path string
	fail bool
}

type watchChildObservationFS struct {
	core.FS
	child string
}

func (fs watchChildObservationFS) ObserveWatch(path string, hold bool) (core.WatchSnapshot, func(), error) {
	if path == fs.child {
		return core.WatchSnapshot{}, nil, os.ErrPermission
	}
	return fs.FS.ObserveWatch(path, hold)
}

func TestWatchDirectoryRegistrationAvoidsTreeObservation(t *testing.T) {
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	f.svc.FS = watchChildObservationFS{FS: f.svc.FS, child: path}
	if err := watch.Reconcile(f.svc); err != nil || len(f.items()) != 1 {
		t.Fatalf("directory date still sampled entire tree: %+v %v", f.items(), err)
	}
	f.sweepAt(2 * time.Hour)
	p := expiredWatchProposal(t, f)
	if p.WatchBurial.Files != 1 || p.Size() != int64(len("downloads/tree/file.zip")) {
		t.Fatalf("directory proposal summary incomplete: %+v", p)
	}
}

func (fs *watchRetryFS) Identify(path string) (core.Identity, error) {
	if fs.fail && samePath(path, fs.path) {
		return core.Identity{}, os.ErrPermission
	}
	return fs.FS.Identify(path)
}

func TestWatchRegistrationKeepsUnknownIdentityErrorsRetryable(t *testing.T) {
	f := newFixture(t)
	path := f.file("downloads/file")
	fs := &watchRetryFS{FS: f.svc.FS, path: path}
	f.svc.FS = fs
	f.svc.Owner = func(p string) (string, bool, error) {
		fs.fail = samePath(p, path)
		return auto.Owner(p)
	}
	p, err := f.svc.SetWatch(filepath.Dir(path), "1d", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	err = f.svc.RegisterWatchChild(p, path)
	if !errors.Is(err, os.ErrPermission) || errors.Is(err, core.ErrUnsafe) || len(f.items()) != 0 {
		t.Fatalf("unknown identity failure lost retry semantics: %v", err)
	}
}

func TestWatchFirstSeenPreservesExpiryAcrossScansAndRenames(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/old")
	old := f.clock.Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.SetWatch(filepath.Dir(path), "30d", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	start := f.clock
	if err := f.svc.RegisterWatchChild(p, path); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(time.Hour)
	renamed := filepath.Join(filepath.Dir(path), "renamed")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RegisterWatchChild(p, renamed); err != nil {
		t.Fatal(err)
	}
	items := f.items()
	if len(items) != 1 || !items[0].ExpiresAt.Equal(start.Add(30*24*time.Hour)) || !items[0].CreatedAt.Equal(start) {
		t.Fatalf("first-seen age reset or used file date: %+v", items)
	}
	if err := f.svc.RemoveWatch(p.Path); err != nil || len(f.items()) != 0 || !exists(renamed) {
		t.Fatalf("remove did not cancel only the lifespan: %v", err)
	}
}

func TestWatchModifiedDatesRequireApprovalAndCanExpireImmediately(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/old")
	old := f.clock.Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.SetWatch(filepath.Dir(path), "1d", core.WatchModified)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RegisterWatchChild(p, path); err == nil || len(f.items()) != 0 {
		t.Fatal("pending date-based policy registered a child")
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	results, err := (&auto.Engine{Svc: f.svc}).Approve([]string{fmt.Sprint(st.Proposals[0].ID)})
	if err != nil || results[0].Error != "" {
		t.Fatalf("approve: %+v %v", results, err)
	}
	if err := f.svc.RegisterWatchChild(p, path); err != nil {
		t.Fatal(err)
	}
	items := f.items()
	if len(items) != 1 || !items[0].ExpiresAt.Equal(old.Add(24*time.Hour)) {
		t.Fatalf("wrong file-date expiry: %+v", items)
	}
	out := f.sweepAt(core.WatchStableWindow)
	if len(out) != 1 || out[0].Action != core.ActionBuried || exists(path) {
		t.Fatalf("approved expired child not buried: %+v", out)
	}
	res, err := f.svc.Resurrect([]string{fmt.Sprint(items[0].ID)})
	if err != nil || len(res) != 1 || res[0].Error != "" || !exists(path) {
		t.Fatalf("watch child not restorable: %+v %v", res, err)
	}
}

func TestWatchCannotShortenManualChildLifespansOrRegisterGrandchildren(t *testing.T) {
	f := newFixture(t)
	f.svc.Owner = auto.Owner
	path := f.file("downloads/sub/file")
	manual := f.add(path, 7*24*time.Hour)
	p, err := f.svc.SetWatch(filepath.Dir(filepath.Dir(path)), "1d", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	if err := f.svc.RegisterWatchChild(p, filepath.Dir(path)); err == nil {
		t.Fatal("policy registered parent with a manual child lifespan")
	}
	newChild := f.file("downloads/sub/other")
	if err := f.svc.RegisterWatchChild(p, newChild); err == nil {
		t.Fatal("policy registered a grandchild")
	}
	items := f.items()
	if len(items) != 1 || !items[0].ExpiresAt.Equal(manual.ExpiresAt) {
		t.Fatalf("manual lifespan changed: %+v", items)
	}
}

func TestWatchActiveDownloadSurvivesExpiry(t *testing.T) {
	for _, name := range []string{"browser.crdownload", "firefox.part", "empty-then-written.zip"} {
		t.Run(name, func(t *testing.T) {
			f, path, _ := watchFixture(t, name, core.WatchFirstSeen)
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			f.clock = f.clock.Add(time.Hour + time.Second)
			if err := os.WriteFile(path, []byte("downloading"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, f.clock, f.clock); err != nil {
				t.Fatal(err)
			}
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			out := f.sweepAt(0)
			if !exists(path) || len(f.burials()) != 0 {
				t.Fatalf("active download buried: %+v", out)
			}
			if filepath.Ext(name) != ".zip" && len(f.items()) != 0 {
				t.Fatal("download temporary name registered")
			}
		})
	}
}

func TestWatchOpenDownloadingHandleSurvivesExpiry(t *testing.T) {
	f, path, _ := watchFixture(t, "active.part", core.WatchFirstSeen)
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(time.Hour + time.Second)
	var written uint32
	if err := windows.WriteFile(h, []byte("still downloading"), &written, nil); err != nil {
		t.Fatal(err)
	}
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("open download buried: %+v", out)
	}
}

func TestWatchModifiedFreshWriteSurvivesOldDeadline(t *testing.T) {
	f, path, _ := watchFixture(t, "file.txt", core.WatchModified, "1d")
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(24*time.Hour + time.Second)
	if err := os.WriteFile(path, []byte("fresh write"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, f.clock, f.clock); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(0)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("modified fresh write buried: %+v", out)
	}
	items := f.items()
	if len(items) != 1 || items[0].ExpiresAt.Before(f.clock.Add(24*time.Hour)) || items[0].Rev <= 1 {
		t.Fatalf("modified deadline was not refreshed: %+v", items)
	}
}

func TestWatchCanceledChildStaysSpared(t *testing.T) {
	f, path, _ := watchFixture(t, "keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Cancel([]string{path})
	if err != nil || core.Failed(rows) {
		t.Fatalf("spare: %+v %v", rows, err)
	}
	// Policy exemptions outlive the write-sensitive automatic exemption.
	// A second write must not silently opt a spared object back into watch.
	f.clock = f.clock.Add(time.Minute)
	if err := os.WriteFile(path, []byte("edited after spare"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, f.clock, f.clock); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	if len(f.items()) != 0 {
		t.Fatal("spared object registered again")
	}
	f.sweepAt(2 * time.Hour)
	if !exists(path) {
		t.Fatal("spared object buried")
	}
}

func TestWatchModifiedResurrectionStaysRestored(t *testing.T) {
	f, path, _ := watchFixture(t, "old.txt", core.WatchModified, "1d")
	old := f.clock.Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(core.WatchStableWindow + time.Minute)
	if len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("bury: %+v", out)
	}
	rows, err := f.svc.Resurrect([]string{path})
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("resurrect: %+v %v", rows, err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	if len(f.items()) != 0 {
		t.Fatal("restored object registered again")
	}
	f.sweepAt(core.WatchStableWindow + time.Minute)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatal("restored object buried again")
	}
}
