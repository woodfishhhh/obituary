package lifecycle_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/testenv"
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
	if f.beforeScan != nil {
		f.beforeScan()
	}
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

func TestAutomaticLargeBurialChecksGitAfterInventory(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and watched directory")
	}
	for _, delay := range []time.Duration{0, 40 * time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { checkLargeBurialGitAge(t, delay) })
		})
	}
}

func checkLargeBurialGitAge(t *testing.T, delay time.Duration) {
	f := newFixture(t)
	testenv.InitRepo(t, f.dir)
	for i := range 288 {
		f.file(fmt.Sprintf("large/%03d", i))
	}
	root := filepath.Join(f.dir, "large")
	f.svc.Owner = auto.Owner
	added, err := f.svc.Add([]string{root}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir})
	if err != nil || len(added) != 1 || core.Failed(added) {
		t.Fatalf("add: %+v %v", added, err)
	}
	gitChecks, finalChecks := 0, 0
	t.Cleanup(core.SetBurialStep(f.svc, func(step string) error {
		if step == "git-checked" {
			gitChecks++
		}
		return nil
	}))
	f.svc.FS = wideInventoryFS{FS: f.fs, beforeCheck: func() {
		if gitChecks != 0 && len(f.fs.recycled) == 0 {
			finalChecks++
			time.Sleep(delay)
		}
	}}
	rows := f.sweepAt(time.Hour)
	if len(rows) != 1 || gitChecks != 1 || finalChecks != 1 {
		t.Fatalf("final Git checks=%d, directory checks before move=%d: %+v", gitChecks, finalChecks, rows)
	}
	moves, active, graves := len(f.fs.recycled), f.items(), f.burials()
	if delay == 0 {
		if rows[0].Action != core.ActionBuried || moves != 1 || exists(root) || len(active) != 0 || len(graves) != 1 {
			t.Fatalf("fast inventory did not bury: moves=%d active=%d graves=%d %+v", moves, len(active), len(graves), rows)
		}
	} else if rows[0].Action != core.ActionRetry || moves != 0 || read(t, filepath.Join(root, "000")) != "large/000" || len(active) != 1 || active[0].ID != added[0].Item.ID || len(graves) != 0 {
		t.Fatalf("expired Git check authorized move: moves=%d active=%d graves=%d %+v", moves, len(active), len(graves), rows)
	}
}
