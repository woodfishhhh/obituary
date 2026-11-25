//go:build windows

package lifecycle_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestMarkRestoreAdviceDoesNotLockLedger(t *testing.T) {
	assertRestoreAdviceDoesNotLockLedger(t, func(f *fixture, b core.Burial) string {
		rows, err := f.svc.Add([]string{b.Path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
		return fmt.Sprint(rows, err)
	})
}

func TestSweepAbsorbsChildrenAtTheirCurrentLocations(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	f := newFixture(t)
	inside := f.file("output/moved-out.txt")
	outside := f.file("moved-in.txt")
	parent := f.add(filepath.Dir(inside), time.Hour)
	leaving, arriving := f.add(inside, time.Minute), f.add(outside, time.Minute)
	kept := filepath.Join(f.dir, "kept.txt")
	if err := os.Rename(inside, kept); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(outside, filepath.Join(parent.Path, "arrived.txt")); err != nil {
		t.Fatal(err)
	}
	// Earlier children are waiting for a retry when their parent is claimed.
	if err := f.svc.Store.Update(func(st *core.State) error {
		for i := range st.Items {
			if st.Items[i].ID != parent.ID {
				st.Items[i].NextAttempt = f.clock.Add(2 * time.Hour)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionBuried || len(f.items()) != 1 || f.items()[0].ID != leaving.ID || read(t, kept) != "output/moved-out.txt" {
		t.Fatalf("wrong child lifespan retired: out=%+v items=%+v", out, f.items())
	}
	if got := containsIDs(t, f.burials()[0]); !reflect.DeepEqual(got, []int64{arriving.ID}) {
		t.Fatalf("contains=%v", got)
	}
}

func TestManifestChildLookupUncertaintyRetries(t *testing.T) {
	for _, cause := range []error{errors.New("offline"), core.ErrUnsafe, core.ErrNotFound} {
		t.Run(cause.Error(), func(t *testing.T) {
			f := newFixture(t)
			child := f.file("output/child.txt")
			dir := filepath.Dir(child)
			f.add(dir, time.Hour)
			f.add(child, 30*24*time.Hour)
			if errors.Is(cause, core.ErrNotFound) {
				if err := os.Remove(child); err != nil {
					t.Fatal(err)
				}
			}
			f.fs.locateErr = cause
			out := f.sweepAt(time.Hour)
			if errors.Is(cause, core.ErrNotFound) {
				if len(out) != 1 || out[0].Action != core.ActionBuried {
					t.Fatalf("definitely absent child prevented burial: %+v", out)
				}
			} else if len(out) != 1 || out[0].Action != core.ActionRetry || !exists(child) || len(f.burials()) != 0 {
				t.Fatalf("uncertain child lookup authorized a move: %+v", out)
			}
		})
	}
}

func TestManifestChildLifespanRecheckedAfterExtension(t *testing.T) {
	f := newFixture(t)
	child := f.file("output/child.txt")
	parent := f.add(filepath.Dir(child), time.Hour)
	childItem := f.add(child, time.Minute)
	f.clock = f.clock.Add(time.Hour)
	restore := core.SetAfterManifest(f.svc, func() {
		if _, err := f.svc.Extend([]string{fmt.Sprint(childItem.ID)}, 30*24*time.Hour); err != nil {
			t.Fatal(err)
		}
	})
	defer restore()
	rows, err := f.svc.Bury([]string{parent.Path}, false, "")
	st, readErr := f.svc.Store.Read()
	idx := st.Index(childItem.ID)
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || readErr != nil || idx < 0 || !exists(child) || len(st.Burials) != 0 || !st.Items[idx].ExpiresAt.After(f.clock) {
		t.Fatalf("extended child lifespan authorized parent burial: rows=%+v state=%+v err=%v/%v", rows, st, err, readErr)
	}
}

func TestManifestChildLifespanRecheckedAfterMoveIntoParent(t *testing.T) {
	f := newFixture(t)
	child := f.file("outside/child.txt")
	parentDir := filepath.Dir(f.file("output/.marker"))
	parent := f.add(parentDir, time.Hour)
	childItem := f.add(child, 30*24*time.Hour)
	moved := filepath.Join(parent.Path, "arrived.txt")
	f.clock = f.clock.Add(time.Hour)
	restore := core.SetAfterManifest(f.svc, func() {
		if err := os.Rename(child, moved); err != nil {
			t.Fatal(err)
		}
		rows, err := f.svc.Add([]string{moved}, core.AddOptions{ExpiresAt: f.clock.Add(30 * 24 * time.Hour)})
		if err != nil || len(rows) != 1 || rows[0].Error != "" {
			t.Fatalf("register moved child: %+v %v", rows, err)
		}
	})
	defer restore()
	rows, err := f.svc.Bury([]string{parent.Path}, false, "")
	st, readErr := f.svc.Store.Read()
	idx := st.Index(childItem.ID)
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || readErr != nil || idx < 0 || !exists(moved) || len(st.Burials) != 0 || st.Items[idx].Path != moved {
		t.Fatalf("moved child lifespan authorized parent burial: rows=%+v state=%+v err=%v/%v", rows, st, err, readErr)
	}
}

func TestSweepContainmentScanDoesNotHoldStoreLock(t *testing.T) {
	testContainmentLock(t, false)
}

func TestSweepMissingUnrelatedDoesNotHoldStoreLock(t *testing.T) {
	testContainmentLock(t, true)
}

func testContainmentLock(t *testing.T, missing bool) {
	if testing.Short() {
		t.Skip("injects thirty slow lookups")
	}
	f := newFixture(t)
	parent := filepath.Dir(f.file("parent/.marker"))
	f.add(parent, time.Hour)
	if missing {
		stale := f.add(f.file("gone.txt"), 30*24*time.Hour)
		if err := os.Remove(stale.Path); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 30 {
		f.add(f.file(fmt.Sprintf("unrelated-%02d.txt", i)), 30*24*time.Hour)
	}
	f.clock = f.clock.Add(time.Hour)
	f.fs.locateDelay = 40 * time.Millisecond
	reader := f.svc.Store
	var maxLock time.Duration
	core.ObserveStoreLocks(&f.svc.Store, func(held time.Duration) {
		if held > maxLock {
			maxLock = held
		}
	})
	readerDone := make(chan time.Duration, 1)
	probe := &burialReaderFS{FS: f.fs, parent: parent}
	f.svc.FS = probe
	t.Cleanup(core.SetAfterManifest(f.svc, func() {
		probe.start = func() {
			entered := make(chan struct{})
			go func() {
				started := time.Now()
				close(entered)
				if _, err := reader.ReadSnapshot(); err != nil {
					t.Error(err)
				}
				readerDone <- time.Since(started)
			}()
			<-entered
		}
	}))
	started := time.Now()
	rows := f.sweepAt(time.Hour)
	elapsed := time.Since(started)
	var readerWait time.Duration
	select {
	case readerWait = <-readerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("reader did not reach the commit")
	}
	if len(rows) != 1 || rows[0].Action != core.ActionBuried || maxLock >= time.Second || readerWait >= time.Second {
		t.Fatalf("containment scan held store.lock: rows=%+v elapsed=%s lock=%s reader=%s", rows, elapsed, maxLock, readerWait)
	}
	t.Logf("containment scan elapsed=%s store.lock=%s reader wait=%s", elapsed, maxLock, readerWait)
}

func TestManifestExternalRenameProtectsChild(t *testing.T) {
	f := newFixture(t)
	src := f.file("outside/child")
	parent := f.add(filepath.Dir(f.file("parent/marker")), time.Hour)
	child := f.add(src, 30*24*time.Hour)
	dst := filepath.Join(parent.Path, "arrived")
	t.Cleanup(core.SetAfterManifest(f.svc, func() {
		if err := os.Rename(src, dst); err != nil {
			t.Fatal(err)
		}
	}))
	rows := f.sweepAt(time.Hour)
	st, err := f.svc.Store.Read()
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || st.Index(child.ID) < 0 || len(st.Burials) != 0 || read(t, dst) != "outside/child" {
		t.Fatalf("physical move lost child: rows=%+v err=%v", rows, err)
	}
}

func TestManifestAliasRegistrationProtectsChild(t *testing.T) {
	if testing.Short() {
		t.Skip("creates a junction via cmd.exe")
	}
	f := newFixture(t)
	path := f.file("parent/child")
	parent := f.add(filepath.Dir(path), time.Hour)
	alias := filepath.Join(f.dir, "alias")
	directoryLink(t, alias, parent.Path)
	t.Cleanup(core.SetAfterManifest(f.svc, func() { f.add(filepath.Join(alias, "child"), 30*24*time.Hour) }))
	rows, err := f.svc.Bury([]string{parent.Path}, false, "")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || len(f.burials()) != 0 || read(t, path) != "parent/child" {
		t.Fatalf("alias registration lost child: rows=%+v err=%v", rows, err)
	}
}

func TestManifestCanceledChildOmitted(t *testing.T) {
	f := newFixture(t)
	path := f.file("parent/child")
	parent := f.add(filepath.Dir(path), time.Hour)
	child := f.add(path, time.Minute)
	f.clock = f.clock.Add(time.Hour)
	t.Cleanup(core.SetAfterManifest(f.svc, func() {
		rows, err := f.svc.Cancel([]string{fmt.Sprint(child.ID)})
		if err != nil || len(rows) != 1 || rows[0].Error != "" {
			t.Fatalf("cancel: %+v %v", rows, err)
		}
	}))
	rows, err := f.svc.Bury([]string{parent.Path}, false, "")
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
		t.Fatalf("parent: %+v %v", rows, err)
	}
	b := f.burials()[0]
	if slices.Contains(b.Contains, child.ID) || len(b.Children) != 0 || read(t, filepath.Join(b.Grave, "child")) != "parent/child" {
		t.Fatalf("canceled child in receipt: %+v", b)
	}
}

func TestSweepEarlierBurialOmittedFromParent(t *testing.T) {
	f := newFixture(t)
	child := f.add(f.file("parent/child"), time.Hour)
	parent := f.add(filepath.Dir(child.Path), time.Hour)
	rows := f.sweepAt(time.Hour)
	bs := f.burials()
	if len(rows) != 2 || len(bs) != 2 || rows[0].Item.ID != child.ID || rows[1].Item.ID != parent.ID || rows[0].Action != core.ActionBuried || rows[1].Action != core.ActionBuried || len(rows[1].Burial.Contains) != 0 || len(rows[1].Burial.Children) != 0 {
		t.Fatalf("earlier child in parent receipt: %+v", rows)
	}
}

type burialReaderFS struct {
	core.FS
	parent string
	start  func()
}

func (f *burialReaderFS) RealPath(path string) (string, error) {
	if f.start != nil && samePath(path, f.parent) {
		f.start()
		f.start = nil
	}
	return f.FS.RealPath(path)
}
