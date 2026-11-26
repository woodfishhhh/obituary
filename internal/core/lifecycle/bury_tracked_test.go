package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
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
