package lifecycle_test

import (
	"path/filepath"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func TestWatchRemovedNestedPolicyStaysExcluded(t *testing.T) {
	f, path, outer := watchFixture(t, "nested/keep.txt", core.WatchFirstSeen)
	inner, err := f.svc.SetWatch(filepath.Dir(path), "7d", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RemoveWatchID(inner.ID); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(2 * time.Hour)
	if len(f.items()) != 0 || !exists(path) {
		t.Fatal("outer policy took over removed nested folder")
	}
	policies, err := f.svc.Watches()
	if err != nil || len(policies) != 1 || len(policies[0].Exclusions) != 1 {
		t.Fatalf("nested exclusion missing: %+v %v", policies, err)
	}
	if err := f.svc.ClearWatchExclusion(outer.ID, inner.ID); err != nil {
		t.Fatal(err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	if len(f.items()) != 1 || f.items()[0].Path != filepath.Dir(path) {
		t.Fatalf("explicit clear did not resume outer policy: %+v", f.items())
	}
}

func TestWatchExclusionBlocksOldClaim(t *testing.T) {
	f, path, p := watchFixture(t, "nested/keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	it := f.items()[0]
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Watches[0].Exclusions = []core.WatchExclusion{{ID: 100, Path: filepath.Dir(path), Identity: *it.Identity}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(2 * time.Hour)
	if !exists(path) || len(f.burials()) != 0 || len(out) != 1 || out[0].Action != core.ActionRefused {
		t.Fatalf("existing claim bypassed watch exclusion: %+v policy=%d", out, p.ID)
	}
}

func TestWatchExclusionsPersistOnPolicyEdit(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Dir(f.file("downloads/nested/keep"))
	outer, err := f.svc.SetWatch(filepath.Dir(dir), "1d", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := f.svc.SetWatch(dir, "7d", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RemoveWatchID(inner.ID); err != nil {
		t.Fatal(err)
	}
	changed, err := f.svc.SetWatch(outer.Path, "2d", core.WatchFirstSeen)
	if err != nil || len(changed.Exclusions) != 1 {
		t.Fatalf("edit lost exclusions: %+v %v", changed, err)
	}
	if err := f.svc.ClearWatchExclusion(outer.ID, inner.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ClearWatchExclusion(outer.ID, inner.ID); err == nil {
		t.Fatalf("missing exclusion %d cleared twice", inner.ID)
	}
}
