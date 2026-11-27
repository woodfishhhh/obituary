package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestContainerBackendLists(t *testing.T) {
	a, _ := fixture(t)
	now := time.Now()
	id := docker.Identity{ID: strings.Repeat("a", 64), ShortID: strings.Repeat("a", 12), Name: "cache", Created: now}
	if err := a.deps.Service.Store.Update(func(st *ledger.State) error {
		st.Items = []ledger.Item{{ID: 1, Kind: ledger.KindContainer, Container: &id, Path: id.Display(), ExpiresAt: now.Add(time.Hour)}}
		st.Burials = []ledger.Burial{{ID: 2, Kind: ledger.KindContainer, Container: &id, Path: id.Display(), BuriedAt: now, PurgeAt: now.Add(time.Hour)}}
		st.Proposals = []ledger.Proposal{{ID: 3, Kind: ledger.ProposalContainer, Candidates: []ledger.Candidate{{Kind: ledger.KindContainer, Container: &id, Path: id.Display(), Lifespan: "2d"}}}}
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
	if err != nil || out.Results[0].Action != ledger.ActionExtended || !out.Results[0].Item.ExpiresAt.Equal(it.ExpiresAt.Add(2*time.Hour)) || out.Results[1].Error == "" {
		t.Fatalf("extend: %+v %v", out, err)
	}
	if _, err := a.Extend([]string{ref(it.ID)}, "wrong"); err == nil {
		t.Fatal("invalid duration accepted")
	}
	if _, err := a.Extend(nil, "2h"); err == nil {
		t.Fatal("empty refs accepted")
	}
	out, err = a.Cancel([]string{ref(it.ID), "999999"})
	if err != nil || out.Results[0].Action != ledger.ActionCanceled || out.Results[1].Error == "" {
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
	if err := a.deps.Service.Store.Update(func(st *ledger.State) error {
		st.Items[0].ExpiresAt = time.Now().Add(-time.Hour)
		st.Items[0].Attempts, st.Items[0].NextAttempt, st.Items[0].LastError = 12, ledger.RetryStopped, "in use"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if s, err := a.Summary(); err != nil || s.DecisionCount != 1 {
		t.Fatalf("a stuck burial is not waiting for a decision: %+v %v", s, err)
	}
	out, err := a.Retry([]string{ref(it.ID)})
	if err != nil || out.Results[0].Action != ledger.ActionRetried || out.Results[0].Item.Stuck() || !out.Results[0].Item.Due(time.Now()) {
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
	if err != nil || len(restored.Results) != 2 || restored.Results[0].Action != ledger.ActionResurrected || restored.Results[1].Action != ledger.ActionResurrected {
		t.Fatalf("undo resurrect: %+v %v", restored, err)
	}
}

func TestBuriedIDsKeepsBuriedRecordWithCleanupError(t *testing.T) {
	ids := buriedIDs([]lifecycle.GraveResult{
		{Ref: "#1", Action: ledger.ActionBuried, Burial: &ledger.Burial{ID: 1}},
		{Ref: "#2", Action: ledger.ActionBuried, Burial: &ledger.Burial{ID: 2}, Error: "manifest locked during cleanup"},
		{Ref: "#3", Action: ledger.ActionRecycled},
		{Ref: "#4", Action: ledger.ActionRefused, Error: "refused"},
	})
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("buriedIDs = %v, want [1 2]", ids)
	}
}

type fakeRecycleFS struct {
	winfs.FS
	recycled []string
}

func (f *fakeRecycleFS) Recycle(path string, _ corefs.Identity) error {
	f.recycled = append(f.recycled, path)
	return os.Remove(path)
}

func TestBuryAndUpcomingStayAlignedInRecycleModeAfterDiskConfigEdit(t *testing.T) {
	a, root := fixture(t)
	rfs := &fakeRecycleFS{}
	a.deps.Service.FS = rfs
	a.deps.Service.Mode = lifecycle.ModeRecycle
	// Externally edit config.toml to graveyard while the live GUI service runs in recycle mode.
	write(t, filepath.Join(a.deps.ConfigDir, "config.toml"), "[graveyard]\nmode = \"graveyard\"\n")

	plain := add(t, a, filepath.Join(root, "plain.txt"), time.Now().Add(time.Hour))
	up, err := a.Upcoming("")
	if err != nil || up.Mode != lifecycle.ModeRecycle {
		t.Fatalf("Upcoming mode = %q, err = %v; want recycle", up.Mode, err)
	}
	reply, err := a.Bury([]string{"#" + ref(plain.ID)})
	if err != nil || len(reply.Results) != 1 || reply.Results[0].Action != ledger.ActionRecycled || len(reply.BuriedIDs) != 0 {
		t.Fatalf("Bury should recycle plain file matching Upcoming.Mode: %+v %v", reply, err)
	}
	if len(rfs.recycled) != 1 {
		t.Fatalf("expected 1 recycled file, got %v", rfs.recycled)
	}
}
