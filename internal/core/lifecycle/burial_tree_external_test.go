package lifecycle_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestMovedDirectoryRequiresCompleteInventory(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Dir(f.file("output/marker"))
	parent := f.add(dir, time.Hour)
	child := f.add(f.file("keep"), 30*24*time.Hour)
	f.fs.onRecycle = func(string) {
		if err := os.Rename(child.Path, filepath.Join(dir, "arrived")); err != nil {
			t.Fatal(err)
		}
	}
	f.clock = f.clock.Add(time.Hour)
	rows, err := f.svc.Bury([]string{dir}, false, "")
	if len(rows) != 1 || rows[0].Action == core.ActionBuried || len(f.burials()) != 0 {
		t.Fatalf("unverified directory became purgeable: %+v %v", rows, err)
	}
	pending, err := f.svc.PendingGraves()
	if err != nil || !slices.ContainsFunc(pending, func(p core.PendingGrave) bool {
		return p.Number() == parent.ID && p.Status == "pending" && p.Burial != nil && read(t, filepath.Join(p.Burial.Grave, "arrived")) == "keep"
	}) {
		t.Fatalf("missing recovery receipt: %+v %v", pending, err)
	}
	if _, err = f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if len(f.burials()) != 0 || len(f.items()) != 2 {
		t.Fatal("recovery released a protected item")
	}
}

func TestMissingUnrelatedPOSIXRegistrationDoesNotBlockBurial(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX has no global inode lookup")
	}
	f := newFixture(t)
	dir := filepath.Dir(f.file("parent/file"))
	f.add(dir, time.Hour)
	other := f.add(f.file("outside"), 24*time.Hour)
	if err := os.Remove(other.Path); err != nil {
		t.Fatal(err)
	}
	rows := f.sweepAt(time.Hour)
	if len(rows) != 1 || rows[0].Action != core.ActionBuried || len(f.items()) != 1 || f.items()[0].ID != other.ID {
		t.Fatalf("unrelated missing record blocked burial: %+v", rows)
	}
}

func TestMovedDirectoryLockBudget(t *testing.T) {
	if runtime.GOOS != "linux" || testing.Short() {
		t.Skip("Linux lock budget")
	}
	f := newFixture(t)
	dir := filepath.Join(f.dir, "large")
	var tracked []string
	for i := range 4144 {
		p := f.file(fmt.Sprintf("large/%04d", i))
		if i%86 == 0 && len(tracked) < 48 {
			tracked = append(tracked, p)
		}
	}
	if rows, err := f.svc.Add(tracked, core.AddOptions{ExpiresAt: f.clock.Add(time.Minute)}); err != nil || core.Failed(rows) {
		t.Fatal(err, rows)
	}
	f.clock = f.clock.Add(time.Hour)
	store := f.svc.Store
	wait := make(chan time.Duration, 1)
	started := make(chan struct{})
	f.fs.onRecycle = func(string) {
		go func() {
			start := time.Now()
			close(started)
			_, e := store.ReadSnapshot()
			if e != nil {
				t.Error(e)
			}
			wait <- time.Since(start)
		}()
		<-started
	}
	var held time.Duration
	core.ObserveStoreLocks(&f.svc.Store, func(d time.Duration) { held = max(held, d) })
	rows, err := f.svc.Bury([]string{dir}, true, "")
	if err != nil || len(rows) != 1 || rows[0].Burial == nil || len(rows[0].Burial.Contains) != 48 {
		t.Fatalf("inventory: %+v %v", rows, err)
	}
	waited := <-wait
	t.Logf("4144 files, 48 children: max store.lock %v, reader %v", held, waited)
	if held >= time.Second || waited >= time.Second {
		t.Fatal("directory inspection exceeded lock budget")
	}
}

type foreignEntryFS struct{ core.FS }

func (f foreignEntryFS) BurialEntry(parent fs.ReadDirFile, path string, dir core.Identity) (core.Identity, bool, error) {
	id, descend, err := f.FS.BurialEntry(parent, path, dir)
	if filepath.Base(path) == "mount" {
		id.Serial++
	}
	return id, descend, err
}
func TestBurialRetainsDirectoryAtDeviceBoundary(t *testing.T) {
	f := newFixture(t)
	file := f.file("parent/mount/marker")
	dir := filepath.Dir(filepath.Dir(file))
	f.add(dir, time.Hour)
	f.svc.FS = foreignEntryFS{f.fs}
	rows := f.sweepAt(time.Hour)
	if len(rows) != 1 || rows[0].Action != core.ActionRetry || read(t, file) != "parent/mount/marker" || len(f.burials()) != 0 {
		t.Fatalf("foreign device authorized burial: %+v", rows)
	}
}

type countedChanges struct {
	corefs.DirectoryChanges
	live *int
}

func (c countedChanges) Close() error { *c.live--; return c.DirectoryChanges.Close() }

type countWatchFS struct {
	core.FS
	live, opened int
}

func (f *countWatchFS) NewDirectoryChanges(p string) (corefs.DirectoryChanges, error) {
	if f.live != 0 {
		return nil, fmt.Errorf("prior directory still subscribed")
	}
	c, err := f.FS.NewDirectoryChanges(p)
	if err != nil {
		return nil, err
	}
	f.live++
	f.opened++
	return countedChanges{c, &f.live}, nil
}
func TestBatchReleasesCompletedDirectoryWatchers(t *testing.T) {
	f := newFixture(t)
	var dirs []string
	for _, name := range []string{"one", "two"} {
		for i := range 275 {
			f.file(fmt.Sprintf("%s/%03d", name, i))
		}
		dirs = append(dirs, filepath.Join(f.dir, name))
	}
	counts := &countWatchFS{FS: f.fs}
	f.svc.FS = counts
	rows, err := f.svc.Bury(dirs, true, "")
	if err != nil || len(rows) != 2 || core.GraveFailed(rows) || counts.opened != 2 || counts.live != 0 {
		t.Fatalf("batch retained directory watchers: %+v %+v %v", counts, rows, err)
	}
}
func TestRecycleDirectoryUsesVerifiableGrave(t *testing.T) {
	f := newFixture(t)
	f.svc.Mode = core.ModeRecycle
	dir := filepath.Dir(f.file("parent/file"))
	rows, err := f.svc.Bury([]string{dir}, true, "")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried || rows[0].Burial == nil || read(t, filepath.Join(rows[0].Burial.Grave, "file")) != "parent/file" {
		t.Fatalf("directory sent to unverifiable recycle bin: %+v %v", rows, err)
	}
}
