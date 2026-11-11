package core_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func TestWatchRemovedPolicyDropsItsExemptions(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, p := watchFixture(t, "keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	spareWatchFixture(t, f, path, false)
	if err := f.svc.RemoveWatchID(p.ID); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Watches) != 0 || len(st.WatchExemptions) != 0 {
		t.Fatalf("removed policy retained orphan exemptions: %+v", st.WatchExemptions)
	}
}

func TestWatchRemoveThroughOriginalParentAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("junction fixture starts cmd.exe")
	}
	f := newFixture(t)
	real := filepath.Join(f.dir, "real")
	path := filepath.Dir(f.file("real/downloads/keep.txt"))
	alias := filepath.Join(f.dir, "alias")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, real).CombinedOutput(); err != nil {
		t.Fatalf("junction: %s %v", out, err)
	}
	input := filepath.Join(alias, "downloads")
	if _, err := f.svc.SetWatch(input, "1d", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RemoveWatch(input); err != nil {
		t.Fatalf("original parent alias could not remove policy: %v", err)
	}
	policies, err := f.svc.Watches()
	if err != nil || len(policies) != 0 || !exists(path) {
		t.Fatalf("alias removal: %+v %v", policies, err)
	}
}

func TestWatchRemoveMissingDirectoryByAliasOrID(t *testing.T) {
	for _, byID := range []bool{false, true} {
		t.Run(fmt.Sprint(byID), func(t *testing.T) {
			f := newFixture(t)
			dir := filepath.Dir(f.file("downloads/keep.txt"))
			p, err := f.svc.SetWatch(dir, "1d", core.WatchFirstSeen)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(dir, dir+"-moved"); err != nil {
				t.Fatal(err)
			}
			if byID {
				err = f.svc.RemoveWatchID(p.ID)
			} else {
				err = f.svc.RemoveWatch(dir)
			}
			if err != nil {
				t.Fatal(err)
			}
			policies, err := f.svc.Watches()
			if err != nil || len(policies) != 0 || !exists(dir+"-moved") {
				t.Fatalf("missing removal: %+v %v", policies, err)
			}
		})
	}
}

func TestWatchDistinctUnicodePolicies(t *testing.T) {
	f := newFixture(t)
	var policies []core.WatchPolicy
	for _, name := range []string{"K", "K"} {
		dir := filepath.Dir(f.file(name + "/keep.txt"))
		p, err := f.svc.SetWatch(dir, "1d", core.WatchFirstSeen)
		if err != nil {
			t.Fatal(err)
		}
		policies = append(policies, p)
	}
	if policies[0].ID == policies[1].ID {
		t.Fatal("Unicode policies merged")
	}
	if err := f.svc.RemoveWatch(policies[1].Path); err != nil {
		t.Fatal(err)
	}
	remain, err := f.svc.Watches()
	if err != nil || len(remain) != 1 || remain[0].ID != policies[0].ID {
		t.Fatalf("Unicode removal changed sibling: %+v %v", remain, err)
	}
}
