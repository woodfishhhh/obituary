package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func TestWatchKeepDetailsRecordReasonPathAndTime(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(fmt.Sprint(restore), func(t *testing.T) {
			f, path, p := watchFixture(t, "keep.txt", core.WatchFirstSeen)
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			spareWatchFixture(t, f, path, restore)
			st, err := f.svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			details := f.svc.WatchExemptionDetails(st, p.ID)
			reason := "spared"
			if restore {
				reason = "restored"
			}
			if len(details) != 1 || details[0].Path != path || details[0].Reason != reason || !details[0].KeptAt.Equal(f.clock) {
				t.Fatalf("keep details: %+v", details)
			}
		})
	}
}

func TestWatchSparedObjectMovedIntoManagedChildStays(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses real Git repositories")
	}
	for _, restore := range []bool{false, true} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("restore=%v/existing=%v", restore, existing), func(t *testing.T) {
				f, path, _ := watchFixture(t, "keep.txt", core.WatchFirstSeen)
				if err := watch.Reconcile(f.svc); err != nil {
					t.Fatal(err)
				}
				spareWatchFixture(t, f, path, restore)
				dest := f.file("downloads/organized/other.txt")
				if existing {
					if err := watch.Reconcile(f.svc); err != nil {
						t.Fatal(err)
					}
				}
				dest = filepath.Join(filepath.Dir(dest), "keep.txt")
				if err := os.Rename(path, dest); err != nil {
					t.Fatal(err)
				}
				_ = watch.Reconcile(f.svc)
				out := f.sweepAt(2 * time.Hour)
				if !exists(dest) || len(f.burials()) != 0 || len(f.items()) != 1 {
					t.Fatalf("spared descendant was registered or buried: %+v", out)
				}
				p := expiredWatchProposal(t, f)
				if !strings.Contains(p.Candidates[0].Rule, "keep.txt") {
					t.Fatal("spared descendant absent from proposal")
				}
			})
		}
	}
}

func TestWatchDescendantExemptionBlocksExistingClaim(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, _ := watchFixture(t, "tree/keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	id, err := f.svc.FS.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.WatchExemptions = []core.WatchExemption{{WatchID: st.Items[0].WatchID, Identity: id}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(2 * time.Hour)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("existing directory claim bypassed descendant exemption: %+v", out)
	}
	if !strings.Contains(expiredWatchProposal(t, f).Candidates[0].Rule, "keep.txt") {
		t.Fatal("existing directory proposal omitted descendant exemption")
	}
}

func TestWatchRestoreAfterPolicyRemovalKeepsExemptionsEmpty(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("watch burial requires Windows writer exclusion; restore policy matching is covered by TestWatchRestoreExemptionRequiresExistingPolicy")
	}
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f, path, p := watchFixture(t, "keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(2 * time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("bury: %+v", out)
	}
	if err := f.svc.RemoveWatchID(p.ID); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Resurrect([]string{fmt.Sprint(out[0].Item.ID)})
	if err != nil || core.GraveFailed(rows) || !exists(path) {
		t.Fatalf("restore: %+v %v", rows, err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.WatchExemptions) != 0 {
		t.Fatalf("restore recreated deleted policy exemption: %+v %v", st.WatchExemptions, err)
	}
}

func TestWatchRestoreExemptionRequiresExistingPolicy(t *testing.T) {
	st := core.State{Watches: []core.WatchPolicy{{ID: 2}}}
	id := core.Identity{FileID: "identity"}
	core.SpareRestoredWatch(&st, core.Burial{WatchID: 1, Identity: &id, Children: []core.BuriedChild{{WatchID: 2, Identity: &id}}})
	if len(st.WatchExemptions) != 1 || st.WatchExemptions[0].WatchID != 2 {
		t.Fatalf("restore recreated orphan exemption: %+v", st.WatchExemptions)
	}
}

func TestWatchExemptionBindsObjectIdentity(t *testing.T) {
	f, path, _ := watchFixture(t, "keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Cancel([]string{path})
	if err != nil || core.Failed(rows) {
		t.Fatalf("spare: %+v %v", rows, err)
	}
	renamed := filepath.Join(filepath.Dir(path), "renamed.txt")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	f.file("downloads/keep.txt")
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	it := f.items()
	if len(it) != 1 || it[0].Path != path {
		t.Fatalf("exemption followed name instead of identity: %+v", it)
	}
	out := f.sweepAt(2 * time.Hour)
	if runtime.GOOS != "windows" {
		requireRetainedRetry(t, f, out)
		if !exists(renamed) || !exists(path) || f.items()[0].Path != path {
			t.Fatal("unsupported watch exclusion lost the exempt original or replacement entry")
		}
		return
	}
	if !exists(renamed) || exists(path) {
		t.Fatal("identity exemption lost or replacement inherited it")
	}
}

func TestWatchExemptionBlocksExistingClaim(t *testing.T) {
	f, path, _ := watchFixture(t, "keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	it := f.items()[0]
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.WatchExemptions = []core.WatchExemption{{WatchID: it.WatchID, Identity: *it.Identity}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(2 * time.Hour)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("existing claim bypassed watch exemption: %+v", out)
	}
}
