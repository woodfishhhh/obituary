package lifecycle_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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
	f.fs.onRecycle = nil
	restored, err := f.svc.Resurrect([]string{fmt.Sprint(parent.ID)})
	if err != nil || len(restored) != 1 || restored[0].Action != core.ActionResurrected || read(t, filepath.Join(dir, "arrived")) != "keep" {
		t.Fatalf("protected tree restore: %+v %v", restored, err)
	}
	if !slices.ContainsFunc(f.items(), func(it core.Item) bool {
		return it.ID == child.ID && *it.Identity == *child.Identity && it.ExpiresAt.Equal(child.ExpiresAt)
	}) {
		t.Fatal("restoration changed the protected registration")
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
	if err != nil || len(rows) != 2 || core.GraveFailed(rows) || counts.opened != 4 || counts.live != 0 {
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

// A receipt-local read fault leaves the original inventory and all registrations intact.
type pendingInventoryFS struct {
	core.FS
	afterMove bool
}

func (f *pendingInventoryFS) MovePinned(p string, id core.Identity, dst string) error {
	err := f.FS.MovePinned(p, id, dst)
	f.afterMove = err == nil
	return err
}
func (f *pendingInventoryFS) OpenBurialDirectory(p string, id core.Identity) (fs.ReadDirFile, error) {
	if f.afterMove {
		f.afterMove = false
		return nil, os.ErrPermission
	}
	return f.FS.OpenBurialDirectory(p, id)
}
func TestPendingDirectoryBatchAndRestore(t *testing.T) {
	f := newFixture(t)
	marker := f.file("dir/marker")
	dir := filepath.Dir(marker)
	parent := f.add(dir, time.Hour)
	keep := f.add(f.file("keep"), 24*time.Hour)
	other := f.file("unrelated")
	f.svc.FS = &pendingInventoryFS{FS: f.fs}
	rows, err := f.svc.Bury([]string{dir, other}, true, "")
	if len(rows) != 2 || rows[0].Action != core.ActionRecoveryPending {
		t.Fatalf("pending: %+v %v", rows, err)
	}
	if err != nil || rows[1].Action != core.ActionBuried {
		t.Errorf("independent request: %+v %v", rows, err)
	}
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "pending" {
		t.Fatalf("pending receipts: %+v %v", pending, err)
	}
	ref := fmt.Sprint(parent.ID)
	grave := pending[0].Burial.Grave
	if err := os.WriteFile(dir, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := f.svc.Resurrect([]string{ref})
	if err != nil || len(restored) != 1 || !strings.Contains(restored[0].Error, grave) || !strings.Contains(restored[0].Error, dir) || read(t, dir) != "occupied" {
		t.Fatalf("conflict: %+v %v", restored, err)
	}
	if err := os.Rename(dir, dir+".saved"); err != nil {
		t.Fatal(err)
	}
	restored, err = f.svc.Resurrect([]string{ref})
	if err != nil || len(restored) != 1 || restored[0].Action != core.ActionResurrected || read(t, marker) != "dir/marker" {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	pending, err = f.svc.PendingGraves()
	if err != nil || len(pending) != 0 || !slices.ContainsFunc(f.items(), func(it core.Item) bool { return it.ID == keep.ID && *it.Identity == *keep.Identity }) {
		t.Fatalf("restore evidence: %+v %v", pending, err)
	}
}
func TestHistoricalDirectoryTypePreservesChild(t *testing.T) {
	for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			c := f.file("parent/child")
			p := f.add(filepath.Dir(c), time.Hour)
			f.add(c, 24*time.Hour)
			f.svc.Mode = mode
			if err := f.svc.Store.UpdateHot(func(st *core.State) error { st.Items[st.Index(p.ID)].IsDir = false; return nil }); err != nil {
				t.Fatal(err)
			}
			f.clock = f.clock.Add(time.Hour)
			for _, dry := range []bool{true, false} {
				rows, err := f.svc.Sweep(dry)
				if err != nil || len(rows) != 1 || rows[0].Action == core.ActionRecycled || rows[0].Action == core.ActionBuried || !exists(c) {
					t.Fatalf("historical type dry=%v: %+v %v", dry, rows, err)
				}
			}
		})
	}
}

func TestPurgeProtectsActiveGraveContent(t *testing.T) {
	for _, entry := range []string{"preview", "selected", "scheduled"} {
		t.Run(entry, func(t *testing.T) {
			f := newFixture(t)
			original := f.file("tree/member")
			f.file("tree/sibling")
			rows, err := f.svc.Bury([]string{filepath.Dir(original)}, true, "")
			if err != nil || core.GraveFailed(rows) {
				t.Fatal(rows, err)
			}
			b := rows[0].Burial
			inside := filepath.Join(b.Grave, "member")
			id, err := f.fs.Identify(inside)
			if err != nil {
				t.Fatal(err)
			}
			f.clock = b.PurgeAt.Add(time.Hour)
			err = f.svc.Store.UpdateHot(func(st *core.State) error {
				st.Items = append(st.Items, core.Item{ID: st.NextID, Rev: 1, Path: original, Identity: &id, ExpiresAt: f.clock.Add(24 * time.Hour)})
				st.NextID++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if entry == "scheduled" {
				_, rows, err = f.svc.Tick()
			} else {
				rows, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, entry == "preview")
			}
			if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "protected") || len(f.burials()) != 1 || f.burials()[0].PurgeIntent != nil {
				t.Fatalf("did not protect active grave member: %+v %v", rows, err)
			}
			if read(t, inside) != "tree/member" || read(t, filepath.Join(b.Grave, "sibling")) != "tree/sibling" {
				t.Fatal("changed protected grave")
			}
		})
	}
}

type dirtyInventoryFS struct {
	core.FS
	grave  string
	closed bool
}

func (f *dirtyInventoryFS) MovePinned(p string, id core.Identity, dst string) error {
	f.grave = dst
	return f.FS.MovePinned(p, id, dst)
}
func (f *dirtyInventoryFS) NewDirectoryChanges(p string) (corefs.DirectoryChanges, error) {
	watch, err := f.FS.NewDirectoryChanges(p)
	if err != nil || p != f.grave {
		return watch, err
	}
	return &dirtyInventory{watch, f}, nil
}

type dirtyInventory struct {
	corefs.DirectoryChanges
	owner *dirtyInventoryFS
}

func (*dirtyInventory) Check() error   { return fmt.Errorf("inventory overflow") }
func (c *dirtyInventory) Close() error { c.owner.closed = true; return c.DirectoryChanges.Close() }
func TestDirtyGraveInventoryRemainsPending(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Dir(f.file("tree/entry"))
	fs := &dirtyInventoryFS{FS: f.fs}
	f.svc.FS = fs
	rows, err := f.svc.Bury([]string{dir}, true, "")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRecoveryPending || !strings.Contains(rows[0].Error, "inventory overflow") || !fs.closed || len(f.burials()) != 0 {
		t.Fatalf("dirty inventory published or watcher leaked: %+v %v", rows, err)
	}
	if got := read(t, filepath.Join(fs.grave, "entry")); got != "tree/entry" {
		t.Fatal(got)
	}
}
