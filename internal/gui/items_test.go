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
