package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestMCPItemVersionTracksReplacement(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "payload.txt")
	clitest.WriteFixture(t, path, "old")
	added, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || core.Failed(added) {
		t.Fatal(added, err)
	}
	var first struct{ Version string }
	clitest.CallMCP(t, s, "obit_list", map[string]any{}, false, &first)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, path, "replacement")
	newRows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || core.Failed(newRows) || newRows[0].Item.ID != added[0].Item.ID || *newRows[0].Item.Identity == *added[0].Item.Identity {
		t.Fatalf("replacement fixture: %+v %v", newRows, err)
	}
	var next struct {
		Changed bool
		Version string
	}
	clitest.CallMCP(t, s, "obit_list", map[string]any{"version": first.Version}, false, &next)
	if !next.Changed || next.Version == first.Version {
		t.Fatal("replacement identity did not change list version")
	}
}

func TestMCPProposalVersionTracksCandidates(t *testing.T) {
	for _, change := range []string{"added", "revision"} {
		t.Run(change, func(t *testing.T) {
			s, repo, _ := mcpFixture(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			store := svc.Store
			a, b := filepath.Join(repo, "a.log"), filepath.Join(repo, "b.log")
			clitest.WriteFixture(t, a, "a")
			clitest.WriteFixture(t, b, "b")
			clitest.AddProposal(t, store, repo, core.Candidate{Path: a, Rev: 1, Lifespan: "1d"})
			var first struct{ Version string }
			clitest.CallMCP(t, s, "obit_proposals", map[string]any{}, false, &first)
			if err := store.Update(func(st *core.State) error {
				if change == "added" {
					st.Proposals[0].Candidates = append(st.Proposals[0].Candidates, core.Candidate{Path: b, Rev: 1})
				} else {
					st.Proposals[0].Candidates[0].Rev++
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			var next struct {
				Changed bool
				Version string
			}
			clitest.CallMCP(t, s, "obit_proposals", map[string]any{"version": first.Version}, false, &next)
			if !next.Changed || next.Version == first.Version {
				t.Fatal("candidate membership did not change list version")
			}
		})
	}
}

func TestMCPVersionExcludesUnrelatedFields(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	store := svc.Store
	path := filepath.Join(repo, "payload.txt")
	clitest.WriteFixture(t, path, "keep")
	if got, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)}); err != nil || core.Failed(got) {
		t.Fatal(got, err)
	}
	clitest.AddProposal(t, store, repo, core.Candidate{Path: path, Rev: 1, Lifespan: "1d"})
	for _, tool := range []string{"obit_list", "obit_proposals"} {
		var first struct{ Version string }
		clitest.CallMCP(t, s, tool, map[string]any{}, false, &first)
		if err := store.Update(func(st *core.State) error {
			st.Items[0].Note = "edited"
			st.Items[0].ExpiresAt = st.Items[0].ExpiresAt.Add(time.Hour)
			st.Items[0].Rev++
			st.Proposals[0].Notified = append(st.Proposals[0].Notified, "session")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var next struct {
			Changed bool
			Version string
		}
		clitest.CallMCP(t, s, tool, map[string]any{"version": first.Version}, false, &next)
		if next.Changed || next.Version != first.Version {
			t.Fatalf("unrelated fields changed list version: %+v", next)
		}
	}
}

func TestMCPListVersions(t *testing.T) {
	for _, tool := range []string{"obit_list", "obit_graveyard", "obit_proposals"} {
		t.Run(tool, func(t *testing.T) {
			s, repo, _ := mcpFixture(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repo, "candidate.log")
			clitest.WriteFixture(t, path, "keep")
			seed := func(st *core.State, id int64) {
				switch tool {
				case "obit_list":
					st.Items = append(st.Items, core.Item{ID: id, Path: path})
				case "obit_graveyard":
					st.Burials = append(st.Burials, core.Burial{ID: id, Path: path})
				case "obit_proposals":
					st.Proposals = append(st.Proposals, core.Proposal{ID: id, Candidates: []core.Candidate{{Path: path}}})
				}
			}
			if err := svc.Store.Update(func(st *core.State) error { seed(st, 3); seed(st, 2); st.NextID = 4; return nil }); err != nil {
				t.Fatal(err)
			}
			var first struct{ Version, NextAfter string }
			clitest.CallMCP(t, s, tool, map[string]any{"limit": 1}, false, &first)
			if len(first.Version) != 16 || first.NextAfter != "2" {
				t.Fatalf("list lacks a stable full-set version: %+v", first)
			}
			for _, changed := range []bool{false, true} {
				if changed {
					if err := svc.Store.Update(func(st *core.State) error { seed(st, 1); return nil }); err != nil {
						t.Fatal(err)
					}
				}
				var page map[string]json.RawMessage
				r := clitest.CallMCP(t, s, tool, map[string]any{"after": first.NextAfter, "version": first.Version}, false, &page)
				var envelope struct{ Warning string }
				if err := json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
					t.Fatal(err)
				}
				var version string
				if err := json.Unmarshal(page["version"], &version); err != nil {
					t.Fatal(err)
				}
				if changed {
					if string(page["changed"]) != "true" || version == first.Version || !strings.Contains(envelope.Warning, "from the beginning") {
						t.Fatalf("changed list was not disclosed: %s warning=%s", page, envelope.Warning)
					}
				} else if page["changed"] != nil || envelope.Warning != "" || version != first.Version {
					t.Fatalf("unchanged list reported a change: %s warning=%s", page, envelope.Warning)
				}
			}
		})
	}
}

func TestMCPGraveyardLateBurialVersion(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	late := filepath.Join(repo, "late.log")
	clitest.WriteFixture(t, late, "late")
	added, err := svc.Add([]string{late}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || core.Failed(added) || added[0].Item.ID != 1 {
		t.Fatal(added, err)
	}
	for _, name := range []string{"second.log", "third.log"} {
		path := filepath.Join(repo, name)
		clitest.WriteFixture(t, path, name)
		got, err := svc.Bury([]string{path}, false, "")
		if err != nil || core.GraveFailed(got) {
			t.Fatal(got, err)
		}
	}
	var first struct{ Version, NextAfter string }
	clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"limit": 1}, false, &first)
	if first.NextAfter != "2" || first.Version == "" {
		t.Fatalf("first page lacks version/cursor: %+v", first)
	}
	if got, err := svc.Bury([]string{late}, false, ""); err != nil || core.GraveFailed(got) || got[0].Burial.ID != 1 {
		t.Fatalf("old ID did not enter the graveyard: %+v %v", got, err)
	}
	var page struct {
		Burials []core.Burial
		Changed bool
		Version string
	}
	r := clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"after": first.NextAfter, "version": first.Version}, false, &page)
	var envelope struct{ Warning string }
	if err := json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
		t.Fatal(err)
	}
	if !page.Changed || page.Version == first.Version || !strings.Contains(envelope.Warning, "from the beginning") || len(page.Burials) != 1 || page.Burials[0].ID != 3 {
		t.Fatalf("late lower-ID burial was silently skipped: %+v warning=%s", page, envelope.Warning)
	}
}

func TestMCPGraveyardVersionTracksPendingStatus(t *testing.T) {
	t.Parallel()
	burials := []core.Burial{{ID: 3}, {ID: 2}}
	pending := []core.PendingGrave{{Burial: &core.Burial{ID: 1}, Manifest: "manifest", Status: "pending"}}
	first := mcpGravePage(burials, pending, mcpPageInput{Limit: 1}, nil)
	version := first.Data.(map[string]any)["version"].(string)
	burials[0], burials[1] = burials[1], burials[0]
	for _, change := range []bool{false, true} {
		if change {
			pending[0].Status = "unverifiable"
		}
		page := mcpGravePage(burials, pending, mcpPageInput{After: "1", Version: version}, nil)
		data := page.Data.(map[string]any)
		if change && (data["changed"] != true || page.Warning == "" || data["version"] == version) {
			t.Fatalf("pending status change behind cursor was hidden: %+v", page)
		}
		if !change && (data["changed"] != nil || page.Warning != "" || data["version"] != version) {
			t.Fatalf("row order changed the version: %+v", page)
		}
	}
}

func TestMCPProposalVersionSurvivesPruneAndReadd(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	a, b := filepath.Join(repo, "a.log"), filepath.Join(repo, "b.log")
	clitest.WriteFixture(t, a, "a")
	clitest.WriteFixture(t, b, "b")
	p, _, _, err := svc.Propose(core.ProposalLarge, repo, "", []core.Candidate{{Path: a, Lifespan: "1d"}, {Path: b, Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	var first struct{ Version string }
	clitest.CallMCP(t, s, "obit_proposals", map[string]any{"limit": 1}, false, &first)
	if err := svc.PruneProposal(p, p.Candidates[:1]); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.Propose(core.ProposalLarge, repo, "", p.Candidates[:1]); err != nil {
		t.Fatal(err)
	}
	var next struct {
		Version string
		Changed bool
	}
	clitest.CallMCP(t, s, "obit_proposals", map[string]any{"after": "1", "version": first.Version}, false, &next)
	if !next.Changed || next.Version == first.Version {
		t.Fatal("readded candidate reused pagination version")
	}
	if _, err := svc.MarkNotified(p.ID, "session"); err != nil {
		t.Fatal(err)
	}
	var notified struct {
		Version string
		Changed bool
	}
	clitest.CallMCP(t, s, "obit_proposals", map[string]any{"after": "1", "version": next.Version}, false, &notified)
	if notified.Changed || notified.Version != next.Version {
		t.Fatal("notification changed proposal membership version")
	}
}
