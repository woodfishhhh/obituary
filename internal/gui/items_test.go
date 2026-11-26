package gui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func TestContainerBackendLists(t *testing.T) {
	a, _ := fixture(t)
	now := time.Now()
	id := docker.Identity{ID: strings.Repeat("a", 64), ShortID: strings.Repeat("a", 12), Name: "cache", Created: now}
	if err := a.deps.Service.Store.Update(func(st *core.State) error {
		st.Items = []core.Item{{ID: 1, Kind: core.KindContainer, Container: &id, Path: id.Display(), ExpiresAt: now.Add(time.Hour)}}
		st.Burials = []core.Burial{{ID: 2, Kind: core.KindContainer, Container: &id, Path: id.Display(), BuriedAt: now, PurgeAt: now.Add(time.Hour)}}
		st.Proposals = []core.Proposal{{ID: 3, Kind: core.ProposalContainer, Candidates: []core.Candidate{{Kind: core.KindContainer, Container: &id, Path: id.Display(), Lifespan: "2d"}}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	items, err := a.Upcoming("")
	if err != nil {
		t.Fatal(err)
	}
	graves, err := a.Graveyard(GraveyardQuery{})
	if err != nil {
		t.Fatal(err)
	}
	proposals, err := a.Proposals()
	if err != nil {
		t.Fatal(err)
	}
	for name, row := range map[string]any{"items": items, "graves": graves, "proposals": proposals} {
		data, err := json.Marshal(row)
		if err != nil || !strings.Contains(string(data), `"kind":"container"`) || !strings.Contains(string(data), "docker:cache") || !strings.Contains(string(data), `"shortId"`) {
			t.Fatalf("container missing from GUI %s: %s %v", name, data, err)
		}
	}
}

func TestUpcomingWindowsAndFailures(t *testing.T) {
	a, root := fixture(t)
	now := time.Now()
	a.deps.Now = func() time.Time { return now }
	add(t, a, filepath.Join(root, "late"), now.Add(8*24*time.Hour))
	soon := add(t, a, filepath.Join(root, "soon"), now.Add(2*time.Hour))
	add(t, a, filepath.Join(root, "week"), now.Add(3*24*time.Hour))
	for window, count := range map[string]int{"": 3, "24h": 1, "7d": 2} {
		out, err := a.Upcoming(window)
		if err != nil || len(out.Items) != count || out.Items[0].ID != soon.ID {
			t.Fatalf("%s: %+v %v", window, out, err)
		}
	}
	if _, err := a.Upcoming("1d"); err == nil {
		t.Fatal("invalid window accepted")
	}
	breakStore(t, a)
	if _, err := a.Upcoming(""); err == nil {
		t.Fatal("store error hidden")
	}
}

func TestExtendAndCancelKeepReceipts(t *testing.T) {
	a, root := fixture(t)
	it := add(t, a, filepath.Join(root, "file"), time.Now().Add(time.Hour))
	out, err := a.Extend([]string{ref(it.ID), "999999"}, "2h")
	if err != nil || out.Results[0].Action != core.ActionExtended || !out.Results[0].Item.ExpiresAt.Equal(it.ExpiresAt.Add(2*time.Hour)) || out.Results[1].Error == "" {
		t.Fatalf("extend: %+v %v", out, err)
	}
	if _, err := a.Extend([]string{ref(it.ID)}, "wrong"); err == nil {
		t.Fatal("invalid duration accepted")
	}
	if _, err := a.Extend(nil, "2h"); err == nil {
		t.Fatal("empty refs accepted")
	}
	out, err = a.Cancel([]string{ref(it.ID), "999999"})
	if err != nil || out.Results[0].Action != core.ActionCanceled || out.Results[1].Error == "" {
		t.Fatalf("cancel: %+v %v", out, err)
	}
	if _, err := a.Cancel(nil); err == nil {
		t.Fatal("empty refs accepted")
	}
	left, err := a.Upcoming("")
	if err != nil || len(left.Items) != 0 {
		t.Fatalf("cancel not persisted: %+v %v", left, err)
	}
}

// A burial that stopped retrying is made due again from the window.
func TestRetryMakesAStuckItemDue(t *testing.T) {
	a, root := fixture(t)
	it := add(t, a, filepath.Join(root, "locked"), time.Now().Add(time.Hour))
	if err := a.deps.Service.Store.Update(func(st *core.State) error {
		st.Items[0].ExpiresAt = time.Now().Add(-time.Hour)
		st.Items[0].Attempts, st.Items[0].NextAttempt, st.Items[0].LastError = 12, core.RetryStopped, "in use"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if s, err := a.Summary(); err != nil || s.DecisionCount != 1 {
		t.Fatalf("a stuck burial is not waiting for a decision: %+v %v", s, err)
	}
	out, err := a.Retry([]string{ref(it.ID)})
	if err != nil || out.Results[0].Action != core.ActionRetried || out.Results[0].Item.Stuck() || !out.Results[0].Item.Due(time.Now()) {
		t.Fatalf("retry: %+v %v", out, err)
	}
	if s, err := a.Summary(); err != nil || s.DecisionCount != 0 {
		t.Fatalf("a retried burial still waits for a decision: %+v %v", s, err)
	}
	if _, err := a.Retry(nil); err == nil {
		t.Fatal("empty refs accepted")
	}
}

func TestBuryBatchUndoPartialAndCrossState(t *testing.T) {
	a, root := fixture(t)
	firstPath := filepath.Join(root, "first.bin")
	secondPath := filepath.Join(root, "second.bin")
	first := add(t, a, firstPath, time.Now().Add(24*time.Hour))
	second := add(t, a, secondPath, time.Now().Add(48*time.Hour))
	write(t, firstPath, strings.Repeat("x", 300))
	write(t, secondPath, strings.Repeat("y", 700))
	already := bury(t, a, filepath.Join(root, "already.bin"))

	if _, err := a.Bury(nil); err == nil {
		t.Fatal("empty refs accepted")
	}
	out, err := a.Bury([]string{"#" + ref(first.ID), "#" + ref(already.ID), "#" + ref(second.ID), "#999999"})
	if err != nil {
		t.Fatalf("Bury: %v", err)
	}
	if len(out.BuriedIDs) != 2 || out.BuriedIDs[0] != first.ID || out.BuriedIDs[1] != second.ID {
		t.Fatalf("unexpected BuriedIDs: %+v", out)
	}
	if out.Results[1].Error == "" || out.Results[3].Error == "" {
		t.Fatalf("cross-state and missing refs must report errors: %+v", out.Results)
	}
	// Verify graveyard rows carry accurate byte sizes for total size calculation.
	page, err := a.Graveyard(GraveyardQuery{})
	if err != nil {
		t.Fatal(err)
	}
	sizes := map[int64]int64{}
	for _, row := range page.Rows {
		if row.Burial != nil {
			sizes[row.ID] = row.Burial.Size
		}
	}
	if sizes[first.ID] != 300 || sizes[second.ID] != 700 {
		t.Fatalf("burial sizes missing: %+v", sizes)
	}
	// Undo via Resurrect restores both newly buried items in place.
	undoRefs := []string{"#" + ref(out.BuriedIDs[0]), "#" + ref(out.BuriedIDs[1])}
	restored, err := a.Resurrect(undoRefs)
	if err != nil || len(restored.Results) != 2 || restored.Results[0].Action != core.ActionResurrected || restored.Results[1].Action != core.ActionResurrected {
		t.Fatalf("undo resurrect: %+v %v", restored, err)
	}
}
