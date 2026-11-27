package lifecycle_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func writePayload(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuryTrackedBatchAndResurrect(t *testing.T) {
	f := newFixture(t)
	fileA := writePayload(t, f.dir, "a.txt", strings.Repeat("a", 512))
	dirB := filepath.Join(f.dir, "folder-b")
	writePayload(t, f.dir, filepath.Join("folder-b", "nested.bin"), strings.Repeat("b", 1024))
	itemA := f.add(fileA, 72*time.Hour)
	itemB := f.add(dirB, 96*time.Hour)

	rows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", itemA.ID), fmt.Sprintf("#%d", itemB.ID)})
	if err != nil {
		t.Fatalf("BuryTracked: %v", err)
	}
	if len(rows) != 2 || rows[0].Action != core.ActionBuried || rows[1].Action != core.ActionBuried {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if rows[0].Burial == nil || rows[0].Burial.ID != itemA.ID || rows[0].Burial.Size != 512 {
		t.Fatalf("burial A mismatch: %+v", rows[0].Burial)
	}
	if rows[1].Burial == nil || rows[1].Burial.ID != itemB.ID || rows[1].Burial.Size != 1024 {
		t.Fatalf("burial B mismatch: %+v", rows[1].Burial)
	}
	if exists(fileA) || exists(dirB) || len(f.items()) != 0 || len(f.burials()) != 2 {
		t.Fatalf("state after bury: items=%d burials=%d", len(f.items()), len(f.burials()))
	}

	restored, err := f.svc.Resurrect([]string{fmt.Sprintf("#%d", itemA.ID), fmt.Sprintf("#%d", itemB.ID)})
	if err != nil || len(restored) != 2 || restored[0].Action != core.ActionResurrected || restored[1].Action != core.ActionResurrected {
		t.Fatalf("Resurrect undo: %+v %v", restored, err)
	}
	if read(t, fileA) != strings.Repeat("a", 512) || read(t, filepath.Join(dirB, "nested.bin")) != strings.Repeat("b", 1024) {
		t.Fatal("restored content mismatch")
	}
}

func TestBuryTrackedRefusesReplacedIdentityAndCrossState(t *testing.T) {
	f := newFixture(t)
	valid := writePayload(t, f.dir, "valid.txt", "keep-buried")
	swapped := writePayload(t, f.dir, "swapped.txt", "original")
	untracked := writePayload(t, f.dir, "untracked.txt", "never-touch")
	already := writePayload(t, f.dir, "already.txt", "buried-first")

	validItem := f.add(valid, 24*time.Hour)
	swappedItem := f.add(swapped, 24*time.Hour)
	buriedRows, err := f.svc.Bury([]string{already}, false, "")
	if err != nil || len(buriedRows) != 1 || buriedRows[0].Burial == nil {
		t.Fatalf("seed burial: %+v %v", buriedRows, err)
	}
	buriedID := buriedRows[0].Burial.ID

	// Replace swapped.txt with a brand-new file (different identity).
	if err := os.Remove(swapped); err != nil {
		t.Fatal(err)
	}
	writePayload(t, f.dir, "swapped.txt", "brand-new-user-file")

	refs := []string{
		fmt.Sprintf("#%d", validItem.ID),
		fmt.Sprintf("#%d", swappedItem.ID),
		fmt.Sprintf("#%d", buriedID),
		untracked,
		fmt.Sprintf("#%d", validItem.ID),
		"#999999",
	}
	rows, err := f.svc.BuryTracked(refs)
	if err != nil {
		t.Fatalf("BuryTracked partial: %v", err)
	}
	if rows[0].Action != core.ActionBuried || rows[0].Error != "" {
		t.Fatalf("valid item should bury: %+v", rows[0])
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].Error == "" || rows[i].Action == core.ActionBuried {
			t.Fatalf("slot %d (%s) should fail: %+v", i, refs[i], rows[i])
		}
	}
	if read(t, swapped) != "brand-new-user-file" {
		t.Fatal("replaced file was touched")
	}
	if read(t, untracked) != "never-touch" {
		t.Fatal("untracked file was touched")
	}
}

func TestBuryTrackedOverlappingParentAndChild(t *testing.T) {
	f := newFixture(t)
	parent := filepath.Join(f.dir, "tree")
	child := writePayload(t, f.dir, filepath.Join("tree", "leaf.txt"), "leaf")
	parentItem := f.add(parent, 48*time.Hour)
	childItem := f.add(child, 24*time.Hour)

	// While child has not expired yet, parent burial must respect child's lifespan.
	rows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", parentItem.ID)})
	if err != nil {
		t.Fatalf("BuryTracked unexpired child: %v", err)
	}
	if rows[0].Error == "" || !exists(parent) {
		t.Fatalf("parent with unexpired child must be refused: %+v", rows[0])
	}

	// Once child is already due, burying [parent, child] absorbs child with parent.
	f.clock = f.clock.Add(25 * time.Hour)
	rows, err = f.svc.BuryTracked([]string{fmt.Sprintf("#%d", parentItem.ID), fmt.Sprintf("#%d", childItem.ID)})
	if err != nil {
		t.Fatalf("BuryTracked overlap: %v", err)
	}
	if rows[0].Action != core.ActionBuried || rows[0].Burial == nil {
		t.Fatalf("parent should bury: %+v", rows[0])
	}
	if rows[1].Error == "" {
		t.Fatalf("child already buried with parent should report error: %+v", rows[1])
	}
	if exists(parent) || len(f.items()) != 0 {
		t.Fatalf("expected parent and child retired: items=%d", len(f.items()))
	}
}

func TestBuryTrackedStopsUnexpiredContainerWhileSweepSkipsIt(t *testing.T) {
	f, d, it := containerFixture(t)
	// Container expires in 1h: Sweep must skip it.
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !d.row.Running || len(f.items()) != 1 || len(f.burials()) != 0 {
		t.Fatalf("Sweep should skip unexpired container: running=%v items=%d burials=%d", d.row.Running, len(f.items()), len(f.burials()))
	}

	// Explicit BuryTracked stops and buries the unexpired container right away.
	rows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", it.ID)})
	if err != nil {
		t.Fatalf("BuryTracked: %v", err)
	}
	if len(rows) != 1 || rows[0].Action != core.ActionBuried || rows[0].Error != "" || rows[0].Burial == nil {
		t.Fatalf("unexpired container should bury: %+v", rows)
	}
	if rows[0].Burial.Reason != core.ReasonBury {
		t.Fatalf("manual container bury receipt reason = %q, want %q", rows[0].Burial.Reason, core.ReasonBury)
	}
	burials := f.burials()
	if d.row.Running || len(f.items()) != 0 || len(burials) != 1 {
		t.Fatalf("container not buried cleanly: running=%v items=%d burials=%d", d.row.Running, len(f.items()), len(burials))
	}
	if burials[0].Reason != core.ReasonBury {
		t.Fatalf("persisted manual container burial reason = %q, want %q", burials[0].Reason, core.ReasonBury)
	}
}

type interruptedAfterStopSent struct {
	*containerStub
	err error
}

func (d interruptedAfterStopSent) Stop(_ context.Context, _ docker.Identity, beforeSend func(docker.Container) error) error {
	if err := beforeSend(d.row); err != nil {
		return err
	}
	d.row.Running = false
	return d.err
}

func TestBuryTrackedContainerInterruptedStopRecoveryPreservesBuryReason(t *testing.T) {
	f, d, it := containerFixture(t)
	f.svc.Docker = interruptedAfterStopSent{containerStub: d, err: errors.New("connection lost after stop sent")}

	// Manual BuryTracked on unexpired container is interrupted after the stop is sent.
	rows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", it.ID)})
	if err != nil {
		t.Fatalf("BuryTracked: %v", err)
	}
	if len(rows) != 1 || rows[0].Action != core.ActionRetry {
		t.Fatalf("expected retry after interrupted stop, got: %+v", rows)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 1 || st.ContainerIntents[0].Reason != core.ReasonBury || len(st.Burials) != 0 {
		t.Fatalf("expected pending bury intent before recovery: intents=%+v burials=%d err=%v", st.ContainerIntents, len(st.Burials), err)
	}

	// Recovery later commits the stopped container and must preserve ReasonBury.
	f.svc.Docker = d
	recovered, err := f.svc.Sweep(false)
	if err != nil {
		t.Fatalf("Sweep recovery: %v", err)
	}
	if len(recovered) != 1 || recovered[0].Action != core.ActionBuried || recovered[0].Burial == nil || recovered[0].Burial.Reason != core.ReasonBury {
		t.Fatalf("recovered manual container bury outcome = %+v", recovered)
	}
	burials := f.burials()
	if len(burials) != 1 || burials[0].Reason != core.ReasonBury {
		t.Fatalf("recovered persisted burial reason = %+v", burials)
	}
}

func TestSweepExpiredContainerRecordsExpiredReason(t *testing.T) {
	f, _, _ := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	rows, err := f.svc.Sweep(false)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(rows) != 1 || rows[0].Action != core.ActionBuried || rows[0].Burial == nil || rows[0].Burial.Reason != core.ReasonExpired {
		t.Fatalf("expired container sweep outcome = %+v", rows)
	}
	burials := f.burials()
	if len(burials) != 1 || burials[0].Reason != core.ReasonExpired {
		t.Fatalf("persisted expired container burial = %+v", burials)
	}
}

func TestBuryTrackedContinuesPastRefusedDockerVolume(t *testing.T) {
	f, client := volumeFixture(t)
	client.refs = []docker.Container{{Identity: docker.Identity{ID: strings.Repeat("a", 64), Name: "holder"}}}
	added, err := f.svc.AddDockerVolume([]string{"cache"}, core.AddOptions{ExpiresAt: f.clock.Add(24 * time.Hour)})
	if err != nil || len(added) != 1 || added[0].Item == nil {
		t.Fatalf("AddDockerVolume: %+v %v", added, err)
	}
	volItem := *added[0].Item
	filePath := writePayload(t, f.dir, "after-volume.txt", "keep-moving")
	fileItem := f.add(filePath, 24*time.Hour)

	rows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", volItem.ID), fmt.Sprintf("#%d", fileItem.ID)})
	if err != nil {
		t.Fatalf("BuryTracked should not abort batch on refused volume: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if rows[0].Action != core.ActionRefused || rows[0].Error == "" || client.removed {
		t.Fatalf("mounted volume should be refused without removal: %+v removed=%v", rows[0], client.removed)
	}
	if rows[1].Action != core.ActionBuried || rows[1].Error != "" || rows[1].Burial == nil || exists(filePath) {
		t.Fatalf("subsequent file should still bury: %+v exists=%v", rows[1], exists(filePath))
	}
}

func TestBuryTrackedEnforcesGraveyardQuotaFIFO(t *testing.T) {
	f := newFixture(t)
	oldFile := writePayload(t, f.dir, "old-tracked.bin", strings.Repeat("o", 3000))
	oldRows, err := f.svc.Bury([]string{oldFile}, false, "")
	if err != nil || core.GraveFailed(oldRows) {
		t.Fatalf("seed old burial: %+v %v", oldRows, err)
	}
	f.clock = f.clock.Add(time.Minute)

	newFile := writePayload(t, f.dir, "new-tracked.bin", strings.Repeat("n", 3000))
	item := f.add(newFile, 24*time.Hour)
	f.svc.GraveyardMaxBytes = 4000

	rows, err := f.svc.BuryTracked([]string{fmt.Sprintf("#%d", item.ID)})
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("BuryTracked: %+v %v", rows, err)
	}
	if len(rows) != 1 || rows[0].Action != core.ActionBuried {
		t.Fatalf("expected buried result: %+v", rows)
	}
	if rows[0].QuotaEviction == nil || rows[0].QuotaEviction.PurgedCount != 1 || rows[0].QuotaEviction.Items[0].ID != oldRows[0].Burial.ID {
		t.Fatalf("BuryTracked must enforce FIFO quota eviction: %+v", rows[0].QuotaEviction)
	}
	if len(f.burials()) != 1 || f.burials()[0].ID != item.ID {
		t.Fatalf("expected only new tracked burial remaining: %+v", f.burials())
	}
}
