//go:build windows

package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

type wideInventoryFS struct {
	core.FS
	beforeScan, beforeCheck func()
}

func (f wideInventoryFS) NewDirectoryChanges(path string) (corefs.DirectoryChanges, error) {
	changes, err := f.FS.NewDirectoryChanges(path)
	if err != nil {
		return nil, err
	}
	f.beforeScan()
	return wideInventoryCheck{changes, f.beforeCheck}, nil
}

type wideInventoryCheck struct {
	corefs.DirectoryChanges
	before func()
}

func (c wideInventoryCheck) Check() error {
	if c.before != nil {
		c.before()
	}
	return c.DirectoryChanges.Check()
}

func TestLargeBurialInventoryYieldsAndRevalidates(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(fmt.Sprint(change), func(t *testing.T) {
			f := newFixture(t)
			parent := f.add(filepath.Dir(f.file("parent/nested/marker")), time.Hour)
			root := filepath.Dir(parent.Path)
			f.add(root, time.Hour)
			outside := f.add(f.file("outside"), 30*24*time.Hour)
			for i := range 275 {
				f.file(fmt.Sprintf("parent/nested/%03d", i))
			}
			f.clock = f.clock.Add(time.Hour)
			starts := 0
			dst := filepath.Join(parent.Path, "arrived")
			f.svc.FS = wideInventoryFS{f.fs, func() {
				starts++
				_, err := f.svc.Store.WithDeadline(time.Now().Add(500 * time.Millisecond)).ReadSnapshot()
				if err != nil {
					t.Fatal("wide inventory held ledger lock", err)
				}
			}, func() {
				if change {
					if err := os.Rename(outside.Path, dst); err != nil {
						t.Fatal(err)
					}
				}
			}}
			rows, err := f.svc.Bury([]string{root}, false, "")
			if err != nil || len(rows) != 1 || starts != 1 {
				t.Fatalf("wide retries=%d outcomes=%+v err=%v", starts, rows, err)
			}
			if change {
				if rows[0].Action != core.ActionRetry || read(t, dst) != "outside" || len(f.burials()) != 0 {
					t.Fatalf("changed inventory authorized burial: %+v", rows)
				}
			} else if rows[0].Action != core.ActionBuried || len(f.burials()[0].Contains) != 1 {
				t.Fatalf("stable inventory: %+v", rows)
			}
		})
	}
}

type unreadableDescendantFS struct {
	core.FS
	blocked string
}

func (f unreadableDescendantFS) Identify(path string) (core.Identity, error) {
	if samePath(path, f.blocked) {
		return core.Identity{}, os.ErrPermission
	}
	return f.FS.Identify(path)
}
func TestBurialUnreadablePhysicalChildRetries(t *testing.T) {
	f := newFixture(t)
	file := f.file("parent/unreadable")
	parent := f.add(filepath.Dir(file), time.Hour)
	f.add(f.file("outside"), 24*time.Hour)
	f.svc.FS = unreadableDescendantFS{f.fs, file}
	rows := f.sweepAt(time.Hour)
	if len(rows) != 1 || rows[0].Action != core.ActionRetry || read(t, file) != "parent/unreadable" || !exists(parent.Path) {
		t.Fatalf("unreadable child authorized burial: %+v", rows)
	}
}
