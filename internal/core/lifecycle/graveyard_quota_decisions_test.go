package lifecycle_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

type failingPurgeGraveFS struct {
	core.FS
	dir  string
	fail bool
	err  error
}

func (f *failingPurgeGraveFS) PurgeGrave(path string, want corefs.Identity, plan corefs.PurgePlan) error {
	if f.fail && (f.dir == "" || samePath(path, f.dir)) {
		return f.err
	}
	return f.FS.PurgeGrave(path, want, plan)
}

func TestQuotaDecisionsBindReviewedIdentityAndPreventTOCTOU(t *testing.T) {
	t.Run("untracked-replace-refuses-approve-extend-and-purge", func(t *testing.T) {
		f := newFixture(t)
		f.svc.GraveyardMaxBytes = 5000

		orig := writeSizedFile(t, f, "swap-target.bin", 7000)
		rows, err := f.svc.Bury([]string{orig}, false, "")
		if err != nil || len(rows) != 1 || rows[0].Action != core.ActionQuotaExceeded {
			t.Fatalf("seed Plan A: %+v %v", rows, err)
		}
		st, err := f.svc.Store.Read()
		if err != nil || len(st.Proposals) != 1 {
			t.Fatalf("proposals: %+v %v", st.Proposals, err)
		}
		p := st.Proposals[0]
		fp := core.ProposalFingerprint(p)

		// Replace the original file at the same path with a brand-new small file B.
		if err := os.Remove(orig); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(orig, []byte("precious-new-file"), 0o600); err != nil {
			t.Fatal(err)
		}

		// Extend must reject with ErrProposalChanged and leave B untracked.
		extRes, err := f.svc.ExtendQuotaProposal(fmt.Sprint(p.ID), fp, 24*time.Hour)
		if !errors.Is(err, core.ErrProposalChanged) || extRes.ErrorCode != core.ErrorCodeProposalChanged {
			t.Fatalf("extend replaced file: res=%+v err=%v, want ErrProposalChanged", extRes, err)
		}
		if len(f.items()) != 0 {
			t.Fatalf("replaced file B must not be registered: %+v", f.items())
		}

		// Purge must reject with ErrProposalChanged and preserve B on disk.
		purgeRes, err := f.svc.PurgeQuotaProposal(fmt.Sprint(p.ID), fp)
		if !errors.Is(err, core.ErrProposalChanged) || purgeRes.ErrorCode != core.ErrorCodeProposalChanged {
			t.Fatalf("purge replaced file: res=%+v err=%v, want ErrProposalChanged", purgeRes, err)
		}
		if read(t, orig) != "precious-new-file" {
			t.Fatal("PurgeQuotaProposal touched replaced file B")
		}

		// Raise limit: Approve must still reject with ErrProposalChanged and preserve B!
		f.svc.GraveyardMaxBytes = 20000
		out, err := f.svc.ApproveQuotaProposal(p)
		if !errors.Is(err, core.ErrProposalChanged) {
			t.Fatalf("approve replaced file: out=%+v err=%v, want ErrProposalChanged", out, err)
		}
		if read(t, orig) != "precious-new-file" || len(f.burials()) != 0 {
			t.Fatalf("ApproveQuotaProposal buried replaced file B: burials=%+v", f.burials())
		}
	})

	t.Run("tracked-item-changed-or-canceled-never-falls-back-to-add", func(t *testing.T) {
		f := newFixture(t)
		f.svc.GraveyardMaxBytes = 5000
		huge := writeSizedFile(t, f, "tracked-huge.bin", 7000)
		it := f.add(huge, time.Hour)
		out := f.sweepAt(time.Hour)
		if len(out) != 1 || out[0].Action != core.ActionQuotaExceeded {
			t.Fatalf("sweep Plan A: %+v", out)
		}
		st, err := f.svc.Store.Read()
		if err != nil || len(st.Proposals) != 1 {
			t.Fatalf("proposals: %+v %v", st.Proposals, err)
		}
		p := st.Proposals[0]
		fp := core.ProposalFingerprint(p)

		// Delete the tracked item directly from hot state (simulating concurrent cancel/change).
		if err := f.svc.Store.UpdateHot(func(st *core.State) error {
			idx := st.Index(it.ID)
			st.Items = append(st.Items[:idx], st.Items[idx+1:]...)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		extRes, err := f.svc.ExtendQuotaProposal(fmt.Sprint(p.ID), fp, 24*time.Hour)
		if !errors.Is(err, core.ErrProposalChanged) || extRes.ErrorCode != core.ErrorCodeProposalChanged {
			t.Fatalf("expected ErrProposalChanged when tracked item vanished, got res=%+v err=%v", extRes, err)
		}
		if len(f.items()) != 0 {
			t.Fatalf("ExtendQuotaProposal must not fall back to Add: items=%+v", f.items())
		}
	})
}

func TestQuotaTemporaryGraveExcludedFromFIFOAndAutoPurged(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 5000

	oldPath := writeSizedFile(t, f, "old-keep.bin", 4000)
	oldRows, err := f.svc.Bury([]string{oldPath}, false, "")
	if err != nil || core.GraveFailed(oldRows) {
		t.Fatalf("bury old: %+v %v", oldRows, err)
	}
	oldBurial := *oldRows[0].Burial
	f.clock = f.clock.Add(time.Minute)

	huge := writeSizedFile(t, f, "huge-temp.bin", 8000)
	rows, err := f.svc.Bury([]string{huge}, false, "")
	if err != nil || rows[0].Action != core.ActionQuotaExceeded {
		t.Fatalf("bury huge: %+v %v", rows, err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 {
		t.Fatalf("proposals: %+v %v", st.Proposals, err)
	}
	p := st.Proposals[0]

	// Inject PurgeGrave failure so the temporary quota-decision grave stays in the ledger.
	failFS := &failingPurgeGraveFS{FS: f.fs, fail: true, err: os.ErrPermission}
	f.svc.FS = failFS
	if _, err := f.svc.PurgeQuotaProposal(fmt.Sprint(p.ID), core.ProposalFingerprint(p)); err == nil {
		t.Fatal("expected PurgeQuotaProposal to fail while PurgeGrave is blocked")
	}

	// Verify temporary grave exists with PurgeAt == now and Source == "quota-decision".
	var tempBurial *core.Burial
	for _, b := range f.burials() {
		if b.ID != oldBurial.ID {
			cp := b
			tempBurial = &cp
		}
	}
	if tempBurial == nil || tempBurial.Source != "quota-decision" || tempBurial.PurgeAt.After(f.clock) {
		t.Fatalf("unexpected temp burial: %+v (now=%v)", tempBurial, f.clock)
	}

	// Quota disk usage and FIFO must exclude the temporary grave and NEVER evict oldBurial.
	count, used, err := f.svc.GraveyardDiskUsage()
	if err != nil || count != 1 || used != 4000 {
		t.Fatalf("GraveyardDiskUsage should exclude temp grave: count=%d used=%d err=%v", count, used, err)
	}
	ev, err := f.svc.EnforceGraveyardQuota(false)
	if err != nil || ev.PurgedCount != 0 || ev.FailedCount != 0 {
		t.Fatalf("FIFO must not touch old grave because of temp grave: ev=%+v err=%v", ev, err)
	}
	if !exists(oldBurial.Grave) {
		t.Fatal("old grave was evicted by temporary quota-decision grave")
	}

	// Clear fault and advance slightly past retry backoff: normal Purge(false, false)
	// finishes ONLY the temporary grave while leaving oldBurial intact.
	failFS.fail = false
	f.clock = f.clock.Add(2 * time.Minute)
	purged, err := f.svc.Purge(false, false)
	if err != nil || core.GraveFailed(purged) {
		t.Fatalf("Purge after unlock: %+v %v", purged, err)
	}
	if len(purged) != 1 || purged[0].Burial == nil || purged[0].Burial.ID != tempBurial.ID {
		t.Fatalf("expected only temp grave #%d purged, got %+v", tempBurial.ID, purged)
	}
	remaining := f.burials()
	if len(remaining) != 1 || remaining[0].ID != oldBurial.ID || !exists(oldBurial.Grave) {
		t.Fatalf("old grave must stay intact after temp grave purge: %+v", remaining)
	}
}

func TestWatchBurialToQuotaNoDoubleOrZombieCardAndPreservesChecks(t *testing.T) {
	t.Run("reject-clears-both-cards-no-zombie", func(t *testing.T) {
		f := newFixture(t)
		dir := filepath.Join(f.dir, "downloads")
		folder := filepath.Join(dir, "big-folder")
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "payload.bin"), bytes.Repeat([]byte("w"), 7000), 0o600); err != nil {
			t.Fatal(err)
		}
		w, err := f.svc.SetWatch(dir, "1h", core.WatchFirstSeen)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := f.svc.Store.Read()
		if err := f.svc.ApproveWatch(st.Proposals[0]); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.ResolveProposal(st.Proposals[0], nil); err != nil {
			t.Fatal(err)
		}
		id, err := f.fs.Identify(folder)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Store.UpdateHot(func(st *core.State) error {
			st.Items = append(st.Items, core.Item{
				ID: st.NextID, Rev: 1, Path: folder, IsDir: true, Identity: &id,
				CreatedAt: f.clock.Add(-2 * time.Hour), ExpiresAt: f.clock.Add(-time.Hour),
				Source: "watch", WatchID: w.ID,
				Watch: &core.WatchTiming{StableSince: f.clock.Add(-2 * time.Hour)},
			})
			st.NextID++
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		// First sweep creates watch-burial proposal.
		f.svc.GraveyardMaxBytes = 5000
		_ = f.sweepAt(0)
		st, _ = f.svc.Store.Read()
		if len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalWatchBurial {
			t.Fatalf("expected 1 watch-burial proposal, got %+v", st.Proposals)
		}

		// Approving watch-burial hits Plan A: drops watch-burial and creates 1 quota proposal retaining WatchBurial.
		eng := &auto.Engine{Svc: f.svc}
		appRows, _ := eng.ApproveReviewed([]string{fmt.Sprint(st.Proposals[0].ID)}, []string{core.ProposalFingerprint(st.Proposals[0])})
		if len(appRows) != 1 || appRows[0].Error == "" {
			t.Fatalf("expected quota overflow error on watch burial approval: %+v", appRows)
		}
		st, _ = f.svc.Store.Read()
		if len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalGraveyardQuota || st.Proposals[0].WatchBurial == nil {
			t.Fatalf("expected exactly 1 quota proposal retaining WatchBurial (no double card), got %+v", st.Proposals)
		}

		// Rejecting quota proposal leaves 0 proposals (no zombie watch-burial) and keeps folder in place.
		qp := st.Proposals[0]
		rejRows, err := f.svc.RejectReviewed([]string{fmt.Sprint(qp.ID)}, []string{core.ProposalFingerprint(qp)})
		if err != nil || len(rejRows) != 1 || rejRows[0].Error != "" {
			t.Fatalf("RejectReviewed: %+v %v", rejRows, err)
		}
		st, _ = f.svc.Store.Read()
		if len(st.Proposals) != 0 || len(st.Items) != 0 || !exists(folder) {
			t.Fatalf("expected 0 proposals/items and folder preserved: proposals=%+v items=%+v", st.Proposals, st.Items)
		}
	})

	t.Run("approve-rechecks-watch-review-and-git-ownership", func(t *testing.T) {
		f := newFixture(t)
		dir := filepath.Join(f.dir, "downloads2")
		folder := filepath.Join(dir, "big-dir")
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "a.bin"), bytes.Repeat([]byte("a"), 7000), 0o600); err != nil {
			t.Fatal(err)
		}
		w, err := f.svc.SetWatch(dir, "1h", core.WatchFirstSeen)
		if err != nil {
			t.Fatal(err)
		}
		st, _ := f.svc.Store.Read()
		if err := f.svc.ApproveWatch(st.Proposals[0]); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.ResolveProposal(st.Proposals[0], nil); err != nil {
			t.Fatal(err)
		}
		id, err := f.fs.Identify(folder)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Store.UpdateHot(func(st *core.State) error {
			st.Items = append(st.Items, core.Item{
				ID: st.NextID, Rev: 1, Path: folder, IsDir: true, Identity: &id,
				CreatedAt: f.clock.Add(-2 * time.Hour), ExpiresAt: f.clock.Add(-time.Hour),
				Source: "watch", WatchID: w.ID,
				Watch: &core.WatchTiming{StableSince: f.clock.Add(-2 * time.Hour)},
			})
			st.NextID++
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		f.svc.GraveyardMaxBytes = 5000
		_ = f.sweepAt(0)
		st, _ = f.svc.Store.Read()
		eng := &auto.Engine{Svc: f.svc}
		_, _ = eng.ApproveReviewed([]string{fmt.Sprint(st.Proposals[0].ID)}, []string{core.ProposalFingerprint(st.Proposals[0])})
		st, _ = f.svc.Store.Read()
		qp := st.Proposals[0]

		// Adding a new file inside the watched folder invalidates WatchBurial review!
		if err := os.WriteFile(filepath.Join(folder, "new.txt"), []byte("unreviewed"), 0o600); err != nil {
			t.Fatal(err)
		}
		f.svc.GraveyardMaxBytes = 20000
		appRows, _ := eng.ApproveReviewed([]string{fmt.Sprint(qp.ID)}, []string{core.ProposalFingerprint(qp)})
		if len(appRows) != 1 || appRows[0].ErrorCode != core.ErrorCodeProposalChanged || !exists(folder) {
			t.Fatalf("modified watch folder must be rejected with ErrProposalChanged: %+v", appRows)
		}

		// Now simulate Git tracking on PurgeQuotaProposal -> must refuse permanent deletion.
		st, _ = f.svc.Store.Read()
		qp = st.Proposals[0]
		f.svc.Owner = func(string) (string, bool, error) { return "", true, nil }
		purgeRes, err := f.svc.PurgeQuotaProposal(fmt.Sprint(qp.ID), core.ProposalFingerprint(qp))
		if err == nil || !strings.Contains(purgeRes.Error, "Git") || !exists(folder) {
			t.Fatalf("Git-tracked automatic watch folder must refuse PurgeQuotaProposal: res=%+v err=%v", purgeRes, err)
		}

		// Clear Git tracking: re-reviewing updated fingerprint obeys platform watch rules.
		f.svc.Owner = nil
		appRows, err = eng.ApproveReviewed([]string{fmt.Sprint(qp.ID)}, []string{core.ProposalFingerprint(qp)})
		if runtime.GOOS != "windows" {
			if err != nil || len(appRows) != 1 || appRows[0].Error == "" || !exists(folder) || len(f.burials()) != 0 {
				t.Fatalf("POSIX watch quota approval must not bypass writer exclusion: %+v %v", appRows, err)
			}
			return
		}
		if err != nil || len(appRows) != 1 || appRows[0].Error != "" || exists(folder) {
			t.Fatalf("re-reviewed watch quota approval should bury: %+v %v", appRows, err)
		}
	})
}

func TestApproveQuotaProposalPropagatesQuotaEviction(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 5000
	oldPath := writeSizedFile(t, f, "old-for-evict.bin", 4000)
	if rows, err := f.svc.Bury([]string{oldPath}, false, ""); err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury old: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Minute)

	huge := writeSizedFile(t, f, "approve-evicts.bin", 6000)
	if rows, err := f.svc.Bury([]string{huge}, false, ""); err != nil || rows[0].Action != core.ActionQuotaExceeded {
		t.Fatalf("Plan A: %+v %v", rows, err)
	}
	st, _ := f.svc.Store.Read()
	p := st.Proposals[0]

	// Raise limit to 7000: fits huge (6000), but old(4000)+huge(6000)=10000 > 7000 -> evicts old!
	f.svc.GraveyardMaxBytes = 7000
	eng := &auto.Engine{Svc: f.svc}
	rows, err := eng.ApproveReviewed([]string{fmt.Sprint(p.ID)}, []string{core.ProposalFingerprint(p)})
	if err != nil || len(rows) != 1 || rows[0].Error != "" {
		t.Fatalf("ApproveReviewed: %+v %v", rows, err)
	}
	if rows[0].QuotaEviction == nil || rows[0].QuotaEviction.PurgedCount != 1 {
		t.Fatalf("ProposalResult must propagate QuotaEviction: %+v", rows[0])
	}
}
