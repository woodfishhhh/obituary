package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestGraveyardSortFilterAndCursor(t *testing.T) {
	a, root := fixture(t)
	first := bury(t, a, filepath.Join(root, "first.log"))
	second := bury(t, a, filepath.Join(root, "second.log"))
	page, err := a.Graveyard(GraveyardQuery{Limit: 1})
	if err != nil || page.Error != "" || len(page.Rows) != 1 || page.Rows[0].ID != second.ID || !page.HasMore {
		t.Fatalf("first page: %+v %v", page, err)
	}
	// Removing the cursor item does not skip the next item as offsets do.
	if _, err := a.Resurrect([]string{ref(second.ID)}); err != nil {
		t.Fatal(err)
	}
	next, err := a.Graveyard(GraveyardQuery{Limit: 1, After: page.NextAfter})
	if err != nil || len(next.Rows) != 1 || next.Rows[0].ID != first.ID || next.HasMore {
		t.Fatalf("next: %+v %v", next, err)
	}
	filtered, err := a.Graveyard(GraveyardQuery{Filter: "FIRST"})
	if err != nil || len(filtered.Rows) != 1 {
		t.Fatalf("filter: %+v %v", filtered, err)
	}
	for _, q := range []GraveyardQuery{{Limit: -1}, {Limit: 201}, {After: "bad"}, {Filter: "changed", After: page.NextAfter}} {
		if _, err := a.Graveyard(q); err == nil {
			t.Fatalf("accepted %+v", q)
		}
	}
	breakStore(t, a)
	if _, err := a.Graveyard(GraveyardQuery{}); err == nil {
		t.Fatal("unreadable store hidden")
	}
}

func TestGraveyardIncludesPendingManifestErrors(t *testing.T) {
	a, root := fixture(t)
	b := bury(t, a, filepath.Join(root, "pending.log"))
	manifest := filepath.Join(filepath.Dir(filepath.Dir(b.Grave)), "999.json")
	write(t, manifest, "broken manifest")
	page, err := a.Graveyard(GraveyardQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.PendingCount == 0 {
		t.Fatalf("pending manifest omitted: %+v", page)
	}
	found := false
	for _, row := range page.Rows {
		if row.Pending != nil && row.Pending.Error != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("manifest failure hidden: %+v", page)
	}
}

func TestGraveyardReceiptPreservesRestoreAndPurgeFailures(t *testing.T) {
	a, root := fixture(t)
	b := bury(t, a, filepath.Join(root, "restore.log"))
	write(t, b.Path, "replacement")
	failed, err := a.Resurrect([]string{ref(b.ID)})
	if err != nil || len(failed.Results) != 1 || failed.Results[0].Action != core.ActionResurrected || strings.EqualFold(failed.Results[0].Path, b.Path) {
		t.Fatalf("alternate restore path: %+v %v", failed, err)
	}
	data, _ := os.ReadFile(b.Path)
	if string(data) != "replacement" {
		t.Fatal("overwrote replacement")
	}
	b = bury(t, a, filepath.Join(root, "restore-ok.log"))
	restored, err := a.Resurrect([]string{ref(b.ID), "999999"})
	if err != nil || restored.Results[0].Action != core.ActionResurrected || !strings.EqualFold(restored.Results[0].Path, b.Path) || restored.Results[1].Error == "" {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	b = bury(t, a, filepath.Join(root, "purge.log"))
	purged, err := a.Purge([]string{ref(b.ID), "999999"})
	if err != nil || purged.Results[0].Action != core.ActionPurged || purged.Results[1].Error == "" {
		t.Fatalf("purge: %+v %v", purged, err)
	}
	if _, err := os.Stat(b.Grave); !os.IsNotExist(err) {
		t.Fatalf("purge did not remove payload: %v", err)
	}
	if _, err := a.Purge(nil); err == nil {
		t.Fatal("empty purge selected all")
	}
	if _, err := a.Resurrect(nil); err == nil {
		t.Fatal("empty restore accepted")
	}
}

func TestGraveyardCursorUsesTimeBeforeID(t *testing.T) {
	a, root := fixture(t)
	b1 := bury(t, a, filepath.Join(root, "older-id.log"))
	b2 := bury(t, a, filepath.Join(root, "newer-id.log"))
	if err := a.deps.Service.Store.Update(func(st *core.State) error {
		for i := range st.Burials {
			if st.Burials[i].ID == b1.ID {
				st.Burials[i].BuriedAt = time.Now().Add(time.Hour)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	page, err := a.Graveyard(GraveyardQuery{Limit: 1})
	if err != nil || page.Rows[0].ID != b1.ID {
		t.Fatalf("time ordering: %+v %v", page, err)
	}
	page, err = a.Graveyard(GraveyardQuery{Limit: 1, After: page.NextAfter})
	if err != nil || page.Rows[0].ID != b2.ID {
		t.Fatalf("cursor ordering: %+v %v", page, err)
	}
}
