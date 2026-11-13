package lifecycle_test

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func TestWatchAllowsDownloadsContainerButProtectsRoots(t *testing.T) {
	f := newFixture(t)
	downloads := filepath.Dir(f.file("downloads/file"))
	f.svc.Guard.Keep = append(f.svc.Guard.Keep, f.dir, downloads)
	f.svc.Guard.WatchAllowed = []string{downloads}
	if _, err := f.svc.SetWatch(downloads, "30d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{f.dir, f.svc.Store.Home, pathutil.VolumeRoot(f.dir)} {
		if _, err := f.svc.SetWatch(path, "1d", core.WatchFirstSeen); err == nil {
			t.Errorf("protected root accepted: %s", path)
		}
	}
	if err := f.svc.Guard.Check(downloads, true); err == nil {
		t.Fatal("watch permission weakened the container's deletion guard")
	}
}

func TestWatchMovedChildAndReplacedFolderStaySafe(t *testing.T) {
	for _, change := range []string{"child-moved", "folder-replaced", "policy-removed"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			f.svc.Owner = auto.Owner
			path := f.file("downloads/file")
			dir := filepath.Dir(path)
			policy, err := f.svc.SetWatch(dir, "1h", core.WatchFirstSeen)
			if err != nil {
				t.Fatal(err)
			}
			approveWatch(t, f.svc)
			if err := f.svc.RegisterWatchChild(policy, path); err != nil {
				t.Fatal(err)
			}
			survivor := path
			switch change {
			case "child-moved":
				survivor = filepath.Join(f.dir, "elsewhere")
				err = os.Rename(path, survivor)
			case "folder-replaced":
				err = os.Rename(dir, dir+"-old")
				if err == nil {
					err = os.Mkdir(dir, 0o700)
				}
				survivor = filepath.Join(dir+"-old", "file")
			case "policy-removed":
				err = f.svc.RemoveWatch(dir)
			}
			if err != nil {
				t.Fatal(err)
			}
			f.sweepAt(2 * time.Hour)
			if !exists(survivor) || len(f.burials()) != 0 || len(f.fs.recycled) != 0 {
				t.Fatalf("moved or disabled policy deleted %s", survivor)
			}
		})
	}
}

func TestWatchExistingParentLifespanCannotBuryPolicy(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Dir(f.file("parent/downloads/file"))
	f.add(filepath.Dir(dir), time.Hour)
	if _, err := f.svc.SetWatch(dir, "30d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(2 * time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRefused || !exists(dir) {
		t.Fatalf("parent of watched folder was moved: %+v", out)
	}
}

func TestWatchMovedChildUnicodeSiblingIsOutsidePolicy(t *testing.T) {
	testenv.RequireDistinctUnicodeNames(t)
	f, path, _ := watchFixture(t, "file.txt", core.WatchFirstSeen)
	k := filepath.Join(f.dir, "K")
	kelvin := filepath.Join(f.dir, "K")
	if err := os.Rename(filepath.Dir(path), k); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(kelvin, 0700); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(k, "file.txt")
	p, err := f.svc.SetWatch(k, "1h", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	if err := f.svc.RegisterWatchChild(p, path); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(kelvin, "file.txt")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(2 * time.Hour)
	if runtime.GOOS != "windows" {
		requireRetainedRetry(t, f, out)
		if !exists(moved) {
			t.Fatal("unlocated child outside the policy was moved")
		}
		return
	}
	if !exists(moved) || len(f.burials()) != 0 || len(out) != 1 || out[0].Action != core.ActionRefused {
		t.Fatalf("Unicode sibling outside policy buried: %+v", out)
	}
}
