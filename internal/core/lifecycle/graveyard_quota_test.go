package lifecycle_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func writeSizedFile(t *testing.T, f *fixture, name string, size int) string {
	t.Helper()
	path := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), size), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGraveyardQuotaDefaultUnlimitedNeverEvicts(t *testing.T) {
	f := newFixture(t)
	if f.svc.GraveyardMaxBytes != 0 {
		t.Fatalf("default max bytes = %d, want 0", f.svc.GraveyardMaxBytes)
	}
	for i := range 3 {
		p := writeSizedFile(t, f, fmt.Sprintf("f-%d.bin", i), 4096)
		rows, err := f.svc.Bury([]string{p}, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatalf("bury %d: %+v %v", i, rows, err)
		}
		f.clock = f.clock.Add(time.Minute)
	}
	if got := len(f.burials()); got != 3 {
		t.Fatalf("burials = %d, want 3", got)
	}
}

func TestGraveyardQuotaUnderLimitDoesNotEvict(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 10000
	for i := range 2 {
		p := writeSizedFile(t, f, fmt.Sprintf("u-%d.bin", i), 4000)
		rows, err := f.svc.Bury([]string{p}, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatalf("bury %d: %+v %v", i, rows, err)
		}
		if rows[0].QuotaEviction != nil {
			t.Fatalf("unexpected eviction under quota: %+v", rows[0].QuotaEviction)
		}
		f.clock = f.clock.Add(time.Minute)
	}
	count, total, err := f.svc.GraveyardDiskUsage()
	if err != nil || count != 2 || total != 8000 {
		t.Fatalf("usage = (%d, %d, %v), want (2, 8000, nil)", count, total, err)
	}
}

func TestGraveyardQuotaStrictFIFOAndAudit(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 5000

	// Two items share the exact same BuriedAt timestamp to verify ID tie-breaker.
	p1 := writeSizedFile(t, f, "oldest-id1.bin", 2000)
	p2 := writeSizedFile(t, f, "same-ts-id2.bin", 2000)
	rows, err := f.svc.Bury([]string{p1, p2}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury initial batch: %+v %v", rows, err)
	}
	id1, id2 := rows[0].Burial.ID, rows[1].Burial.ID
	if id1 >= id2 {
		t.Fatalf("expected ascending IDs, got %d, %d", id1, id2)
	}
	grave1 := rows[0].Burial.Grave

	// Bury a third item 1 minute later (total 6000 > 5000): must evict id1 only.
	f.clock = f.clock.Add(time.Minute)
	p3 := writeSizedFile(t, f, "newer-id3.bin", 2000)
	rows3, err := f.svc.Bury([]string{p3}, false, "")
	if err != nil || core.GraveFailed(rows3) {
		t.Fatalf("bury p3: %+v %v", rows3, err)
	}
	ev := rows3[0].QuotaEviction
	if ev == nil || ev.PurgedCount != 1 || ev.FreedBytes != 2000 || ev.AfterBytes != 4000 || ev.MaxBytes != 5000 {
		t.Fatalf("unexpected eviction summary: %+v", ev)
	}
	if len(ev.Items) != 1 || ev.Items[0].ID != id1 {
		t.Fatalf("evicted items = %+v, want only #%d", ev.Items, id1)
	}
	if exists(grave1) || exists(filepath.Dir(grave1)) {
		t.Fatalf("evicted grave still exists on disk: %s", grave1)
	}
	var remainingIDs []int64
	for _, b := range f.burials() {
		remainingIDs = append(remainingIDs, b.ID)
	}
	if !slices.Equal(remainingIDs, []int64{id2, rows3[0].Burial.ID}) {
		t.Fatalf("remaining burials = %v, want [%d %d]", remainingIDs, id2, rows3[0].Burial.ID)
	}
	events, err := f.svc.Store.History(50)
	if err != nil {
		t.Fatal(err)
	}
	foundAudit := false
	for _, e := range events {
		if e.Action == core.ActionPurged && e.ID == id1 && strings.Contains(e.Detail, core.PurgeBasisQuotaFIFO) {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("missing %s audit event for #%d in %+v", core.PurgeBasisQuotaFIFO, id1, events)
	}
}

func TestGraveyardQuotaDirectorySizeDoesNotFollowLinks(t *testing.T) {
	f := newFixture(t)
	// Outside target is huge (20 KB), linked inside a tiny 1 KB directory.
	outside := filepath.Join(t.TempDir(), "outside-huge")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "giant.bin"), bytes.Repeat([]byte("z"), 20000), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.dir, "linked-folder")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "small.bin"), bytes.Repeat([]byte("s"), 1024), 0o600); err != nil {
		t.Fatal(err)
	}
	directoryLink(t, filepath.Join(dir, "escape-link"), outside)

	f.svc.GraveyardMaxBytes = 5000
	rows, err := f.svc.Bury([]string{dir}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("small dir with link should bury without counting outside target: %+v %v", rows, err)
	}
	if rows[0].Burial == nil || rows[0].Burial.Size != 1024 {
		t.Fatalf("burial size = %+v, want 1024", rows[0].Burial)
	}
	// Verify outside file was completely untouched.
	if !exists(filepath.Join(outside, "giant.bin")) {
		t.Fatal("outside linked file disappeared")
	}
}

func TestGraveyardQuotaPlanAOversizedItemPreservesOldGraves(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 5000

	// 1. Existing 4000B file in graveyard (models 4GB old graveyard under 5GB limit).
	oldFile := writeSizedFile(t, f, "old-4k.bin", 4000)
	oldRows, err := f.svc.Bury([]string{oldFile}, false, "")
	if err != nil || core.GraveFailed(oldRows) {
		t.Fatalf("bury old: %+v %v", oldRows, err)
	}
	oldBurial := *oldRows[0].Burial
	f.clock = f.clock.Add(time.Minute)

	// 2. Oversized 8000B file (models 8GB file > 5GB limit) via explicit bury.
	hugeFile := writeSizedFile(t, f, "huge-8k.bin", 8000)
	rows, err := f.svc.Bury([]string{hugeFile}, false, "")
	if err != nil {
		t.Fatalf("unexpected ledger error on Plan A: %v", err)
	}
	if len(rows) != 1 || rows[0].Action != core.ActionQuotaExceeded || rows[0].QuotaOverflow == nil {
		t.Fatalf("expected Plan A quota-exceeded receipt, got %+v", rows)
	}
	if rows[0].QuotaOverflow.ItemBytes != 8000 || rows[0].QuotaOverflow.MaxBytes != 5000 || rows[0].QuotaOverflow.ProposalID == 0 {
		t.Fatalf("unexpected quotaOverflow payload: %+v", rows[0].QuotaOverflow)
	}
	if rows[0].QuotaEviction != nil {
		t.Fatalf("Plan A must never trigger FIFO eviction: %+v", rows[0].QuotaEviction)
	}
	// 3. Oversized file stayed safely in place; old 4KB grave is 100% intact.
	if !exists(hugeFile) {
		t.Fatal("oversized file was removed instead of staying in place")
	}
	if !exists(oldBurial.Grave) {
		t.Fatal("existing old grave was evicted when Plan A triggered")
	}
	burials := f.burials()
	if len(burials) != 1 || burials[0].ID != oldBurial.ID {
		t.Fatalf("burials changed under Plan A: %+v", burials)
	}
	// 4. Appears in pending decisions with clear reason & QuotaReview.
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalGraveyardQuota || st.Proposals[0].QuotaReview == nil {
		t.Fatalf("expected 1 graveyard-quota proposal, got %+v", st.Proposals)
	}
	if !strings.Contains(st.Proposals[0].Candidates[0].Rule, "超出墓地容量上限") {
		t.Fatalf("unexpected proposal rule: %q", st.Proposals[0].Candidates[0].Rule)
	}
	// 5. Audit log explicitly recorded quota-exceeded (not buried/purged).
	events, err := f.svc.Store.History(50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Action == core.ActionQuotaExceeded && strings.Contains(e.Detail, "graveyard_capacity_exceeded") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing quota-exceeded audit event in %+v", events)
	}
}

func TestGraveyardQuotaSweepPlanADeduplicatesAndUnblocksOnLimitRaise(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 5000
	huge := writeSizedFile(t, f, "sweep-huge.bin", 7000)
	f.add(huge, time.Hour)

	out1 := f.sweepAt(time.Hour)
	if len(out1) != 1 || out1[0].Action != core.ActionQuotaExceeded || out1[0].QuotaOverflow == nil {
		t.Fatalf("first sweep = %+v, want quota-exceeded", out1)
	}
	if !exists(huge) || len(f.items()) != 1 {
		t.Fatalf("tracked item must stay in place and tracked while awaiting decision")
	}

	// Second sweep while limit unchanged skips cleanly without duplicating proposal.
	out2 := f.sweepAt(time.Hour)
	if len(out2) != 0 {
		t.Fatalf("second sweep should skip already-pending quota decision, got %+v", out2)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Proposals) != 1 {
		t.Fatalf("expected 1 deduped proposal, got %d", len(st.Proposals))
	}

	// Raising limit above item size unblocks sweep and drops the quota proposal.
	f.svc.GraveyardMaxBytes = 10000
	out3 := f.sweepAt(time.Minute)
	if len(out3) != 1 || out3[0].Action != core.ActionBuried {
		t.Fatalf("sweep after raising limit = %+v, want buried", out3)
	}
	st, err = f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Proposals) != 0 {
		t.Fatalf("quota proposal should be dropped after burial, got %+v", st.Proposals)
	}
}

func TestGraveyardQuotaPurgeFailureSkipsToNextOldestWithoutLoop(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 100000

	p1 := writeSizedFile(t, f, "a-oldest.bin", 3000)
	r1, err := f.svc.Bury([]string{p1}, false, "")
	if err != nil || core.GraveFailed(r1) {
		t.Fatalf("bury p1: %+v %v", r1, err)
	}
	f.clock = f.clock.Add(time.Minute)

	p2 := writeSizedFile(t, f, "b-second.bin", 3000)
	r2, err := f.svc.Bury([]string{p2}, false, "")
	if err != nil || core.GraveFailed(r2) {
		t.Fatalf("bury p2: %+v %v", r2, err)
	}
	f.clock = f.clock.Add(time.Minute)

	p3 := writeSizedFile(t, f, "c-newest.bin", 3000)
	r3, err := f.svc.Bury([]string{p3}, false, "")
	if err != nil || core.GraveFailed(r3) {
		t.Fatalf("bury p3: %+v %v", r3, err)
	}

	// Make the oldest grave unreadable (permission denied) to simulate locked/unpurgeable item.
	f.svc.FS = unreadableGraveFS{FS: f.fs, dir: filepath.Dir(r1[0].Burial.Grave)}
	f.svc.GraveyardMaxBytes = 6000
	ev, err := f.svc.EnforceGraveyardQuota(false)
	if err != nil {
		t.Fatalf("enforce error: %v", err)
	}
	if ev.FailedCount != 1 || ev.PurgedCount != 1 || len(ev.Items) != 1 || ev.Items[0].ID != r2[0].Burial.ID {
		t.Fatalf("expected p1 failed and p2 evicted, got %+v", ev)
	}
	if !exists(r1[0].Burial.Grave) {
		t.Fatal("failed oldest grave must remain intact")
	}
	if exists(r2[0].Burial.Grave) {
		t.Fatal("second oldest grave should have been evicted")
	}
	if !exists(r3[0].Burial.Grave) {
		t.Fatal("newest grave should remain intact")
	}
}

func TestGraveyardQuotaPreviewIsReadOnlyAndLowerLimitEnforces(t *testing.T) {
	f := newFixture(t)
	var ids []int64
	var graves []string
	for i := range 3 {
		p := writeSizedFile(t, f, fmt.Sprintf("p-%d.bin", i), 2000)
		rows, err := f.svc.Bury([]string{p}, false, "")
		if err != nil || core.GraveFailed(rows) {
			t.Fatalf("bury %d: %+v %v", i, rows, err)
		}
		ids = append(ids, rows[0].Burial.ID)
		graves = append(graves, rows[0].Burial.Grave)
		f.clock = f.clock.Add(time.Minute)
	}

	preview, err := f.svc.PreviewGraveyardQuotaEviction(2500)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.BeforeBytes != 6000 || preview.PurgedCount != 2 || preview.FreedBytes != 4000 || preview.AfterBytes != 2000 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if len(preview.Items) != 2 || preview.Items[0].ID != ids[0] || preview.Items[1].ID != ids[1] {
		t.Fatalf("preview order wrong: %+v", preview.Items)
	}
	// Preview must not delete anything or alter ledger.
	if len(f.burials()) != 3 {
		t.Fatalf("preview altered ledger burials: %d", len(f.burials()))
	}
	for _, g := range graves {
		if !exists(g) {
			t.Fatalf("preview deleted grave %s", g)
		}
	}

	// Now actually lower limit and enforce.
	f.svc.GraveyardMaxBytes = 2500
	ev, err := f.svc.EnforceGraveyardQuota(false)
	if err != nil {
		t.Fatalf("enforce: %v", err)
	}
	if ev.PurgedCount != 2 || ev.AfterBytes != 2000 {
		t.Fatalf("enforce = %+v, want 2 purged and 2000 remaining", ev)
	}
	remaining := f.burials()
	if len(remaining) != 1 || remaining[0].ID != ids[2] {
		t.Fatalf("remaining burials = %+v, want only #%d", remaining, ids[2])
	}
}

func TestGraveyardQuotaDecisionsPurgeExtendRejectAndApprove(t *testing.T) {
	t.Run("permanent-delete-preserves-old-graves", func(t *testing.T) {
		f := newFixture(t)
		f.svc.GraveyardMaxBytes = 5000
		oldPath := writeSizedFile(t, f, "keep-old.bin", 4000)
		oldRows, err := f.svc.Bury([]string{oldPath}, false, "")
		if err != nil || core.GraveFailed(oldRows) {
			t.Fatalf("bury old: %+v %v", oldRows, err)
		}
		hugePath := writeSizedFile(t, f, "delete-huge.bin", 8000)
		rows, err := f.svc.Bury([]string{hugePath}, false, "")
		if err != nil || rows[0].Action != core.ActionQuotaExceeded {
			t.Fatalf("bury huge: %+v %v", rows, err)
		}
		st, err := f.svc.Store.Read()
		if err != nil || len(st.Proposals) != 1 {
			t.Fatalf("proposals: %+v %v", st.Proposals, err)
		}
		p := st.Proposals[0]
		res, err := f.svc.PurgeQuotaProposal(fmt.Sprint(p.ID), core.ProposalFingerprint(p))
		if err != nil || res.Error != "" {
			t.Fatalf("PurgeQuotaProposal: %+v %v", res, err)
		}
		if exists(hugePath) {
			t.Fatal("oversized file should be permanently deleted after explicit confirmation")
		}
		if !exists(oldRows[0].Burial.Grave) || len(f.burials()) != 1 {
			t.Fatalf("old grave must stay completely untouched, burials=%+v", f.burials())
		}
		st, err = f.svc.Store.Read()
		if err != nil || len(st.Proposals) != 0 {
			t.Fatalf("proposal should be cleared, got %+v %v", st.Proposals, err)
		}
	})

	t.Run("extend-and-reject-and-approve", func(t *testing.T) {
		f := newFixture(t)
		f.svc.GraveyardMaxBytes = 5000
		huge1 := writeSizedFile(t, f, "ext.bin", 7000)
		_, _ = f.svc.Bury([]string{huge1}, false, "")
		st, _ := f.svc.Store.Read()
		p1 := st.Proposals[0]
		res, err := f.svc.ExtendQuotaProposal(fmt.Sprint(p1.ID), core.ProposalFingerprint(p1), 24*time.Hour)
		if err != nil || res.Error != "" {
			t.Fatalf("ExtendQuotaProposal: %+v %v", res, err)
		}
		if !exists(huge1) || len(f.items()) != 1 {
			t.Fatalf("extended file should remain in place and tracked")
		}

		// Reject tracked oversized item -> stops tracking, file untouched.
		huge2 := writeSizedFile(t, f, "rej.bin", 7000)
		f.add(huge2, time.Hour)
		_ = f.sweepAt(time.Hour)
		st, _ = f.svc.Store.Read()
		p2 := st.Proposals[0]
		rejRows, err := f.svc.RejectReviewed([]string{fmt.Sprint(p2.ID)}, []string{core.ProposalFingerprint(p2)})
		if err != nil || len(rejRows) != 1 || rejRows[0].Error != "" {
			t.Fatalf("RejectReviewed: %+v %v", rejRows, err)
		}
		if !exists(huge2) {
			t.Fatal("rejected file must stay in place")
		}
		for _, it := range f.items() {
			if samePath(it.Path, huge2) {
				t.Fatal("rejected item must no longer be tracked")
			}
		}

		// Approve while still over limit fails safely without deleting.
		huge3 := writeSizedFile(t, f, "app.bin", 7000)
		_, _ = f.svc.Bury([]string{huge3}, false, "")
		st, _ = f.svc.Store.Read()
		p3 := st.Proposals[0]
		eng := &auto.Engine{Svc: f.svc}
		appRows, _ := eng.ApproveReviewed([]string{fmt.Sprint(p3.ID)}, []string{core.ProposalFingerprint(p3)})
		if len(appRows) != 1 || appRows[0].Error == "" || !exists(huge3) {
			t.Fatalf("approve while still oversized must refuse safely: %+v", appRows)
		}
		// Raise limit -> approve succeeds and buries.
		f.svc.GraveyardMaxBytes = 20000
		appRows, err = eng.ApproveReviewed([]string{fmt.Sprint(p3.ID)}, []string{core.ProposalFingerprint(p3)})
		if err != nil || len(appRows) != 1 || appRows[0].Error != "" || exists(huge3) {
			t.Fatalf("approve after raising limit should bury: %+v %v", appRows, err)
		}
	})
}

func TestGraveyardQuotaExcludesContainersAndRunsInPurge(t *testing.T) {
	f := newFixture(t)
	// Inject a container burial with non-zero fake size to prove non-local kinds are never counted or FIFO-evicted.
	if err := f.svc.Store.UpdateGraveyard(func(st *core.State) error {
		st.Burials = append(st.Burials, core.Burial{
			ID: 999, Kind: core.KindContainer, Path: "docker-ctr", Size: 999999,
			Container: &docker.Identity{ID: strings.Repeat("a", 64), ShortID: strings.Repeat("a", 12), Name: "docker-ctr", Created: f.clock.Add(-48 * time.Hour)},
			BuriedAt:  f.clock.Add(-48 * time.Hour), PurgeAt: f.clock.Add(48 * time.Hour), Reason: core.ReasonBury,
		})
		st.NextID = 1000
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p1 := writeSizedFile(t, f, "local1.bin", 3000)
	p2 := writeSizedFile(t, f, "local2.bin", 3000)
	if rows, err := f.svc.Bury([]string{p1}, false, ""); err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury p1: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Minute)
	if rows, err := f.svc.Bury([]string{p2}, false, ""); err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury p2: %+v %v", rows, err)
	}

	count, bytesUsed, err := f.svc.GraveyardDiskUsage()
	if err != nil || count != 2 || bytesUsed != 6000 {
		t.Fatalf("disk usage = (%d, %d, %v), want (2, 6000, nil)", count, bytesUsed, err)
	}

	// Set limit and trigger via Purge(false, false) (periodic maintenance path).
	f.svc.GraveyardMaxBytes = 4000
	purged, err := f.svc.Purge(false, false)
	if err != nil || core.GraveFailed(purged) {
		t.Fatalf("Purge: %+v %v", purged, err)
	}
	if len(purged) != 1 || purged[0].Detail != core.PurgeBasisQuotaFIFO || purged[0].QuotaEviction == nil {
		t.Fatalf("expected 1 FIFO eviction from Purge, got %+v", purged)
	}
	// Container burial must remain untouched.
	foundContainer := false
	for _, b := range f.burials() {
		if b.ID == 999 {
			foundContainer = true
		}
	}
	if !foundContainer {
		t.Fatal("container burial was mistakenly evicted by local disk quota")
	}
}

func TestGraveyardQuotaConcurrentBuryAndPurgeSafe(t *testing.T) {
	f := newFixture(t)
	f.svc.GraveyardMaxBytes = 4000
	var paths []string
	for i := range 6 {
		paths = append(paths, writeSizedFile(t, f, fmt.Sprintf("race-%d.bin", i), 1500))
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(paths)+2)
	for _, p := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			rows, err := f.svc.Bury([]string{path}, false, "")
			if err != nil {
				errs <- err
				return
			}
			if core.GraveFailed(rows) {
				errs <- fmt.Errorf("bury %s failed: %+v", path, rows)
			}
		}(p)
	}
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.Purge(false, false); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent error: %v", err)
	}
	_, total, err := f.svc.GraveyardDiskUsage()
	if err != nil {
		t.Fatal(err)
	}
	if total > f.svc.GraveyardMaxBytes {
		t.Fatalf("final usage %d exceeded limit %d", total, f.svc.GraveyardMaxBytes)
	}
}
