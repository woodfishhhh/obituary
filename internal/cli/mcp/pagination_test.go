//go:build windows

package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestMCPGraveyardKeepsBoundaryIDGroup(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.Update(func(st *ledger.State) error {
		for i := int64(1); i <= 201; i++ {
			st.Burials = append(st.Burials, ledger.Burial{ID: i, Path: filepath.Join(repo, fmt.Sprint(i))})
		}
		st.NextID = 202
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, filepath.Join(svc.Store.Home, "graveyard", "200-broken.json"), "broken manifest")
	for _, after := range []string{"", "199"} {
		r, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: "obit_graveyard", Arguments: map[string]any{"after": after}})
		if err != nil || r.IsError {
			t.Fatalf("boundary ID group blocked the whole page: %+v %v", r, err)
		}
		var envelope struct {
			Data struct {
				Burials   []ledger.Burial
				Pending   []core.PendingGrave
				NextAfter string
				Total     int
				HasMore   bool
			}
		}
		if err := json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
			t.Fatal(err)
		}
		page := envelope.Data
		if after == "" {
			if len(page.Burials) != 199 || len(page.Pending) != 0 || page.NextAfter != "199" || !page.HasMore || page.Total != 202 {
				t.Fatalf("boundary ID group was split: %+v", page)
			}
		} else if len(page.Burials) != 2 || page.Burials[0].ID != 200 || len(page.Pending) != 1 || page.HasMore || page.NextAfter != "" {
			t.Fatalf("deferred ID group was not returned together: %+v", page)
		}
	}
}

func TestMCPGraveyardSingleIDGroupLimit(t *testing.T) {
	t.Parallel()
	for _, count := range []int{2, 200, 201} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			t.Parallel()
			pending := make([]core.PendingGrave, count)
			for i := range pending {
				pending[i] = core.PendingGrave{Burial: &ledger.Burial{ID: 1}, Status: "unverifiable"}
			}
			page := mcpGravePage(nil, pending, mcpPageInput{Limit: 1}, nil)
			if count > 200 {
				if !strings.Contains(page.Error, "too many records share one ID") {
					t.Fatalf("oversized single ID group was accepted: %+v", page)
				}
			} else if page.Error != "" || len(page.Data.(map[string]any)["pending"].([]core.PendingGrave)) != count {
				t.Fatalf("single ID group was split: %+v", page)
			}
		})
	}
}

func TestMCPListPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tool := range []string{"obit_list", "obit_graveyard", "obit_proposals"} {
		t.Run(tool, func(t *testing.T) {
			s, repo, _ := mcpFixture(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repo, "candidate.log")
			clitest.WriteFixture(t, path, "pending")
			key := "items"
			if tool == "obit_graveyard" {
				key = "burials"
			}
			if tool == "obit_proposals" {
				key = "proposals"
			}
			if err := svc.Store.Update(func(st *ledger.State) error {
				for i := range 205 {
					id := int64(205 - i)
					switch tool {
					case "obit_list":
						st.Items = append(st.Items, ledger.Item{ID: id, Path: path, ExpiresAt: time.Now().Add(time.Duration(i) * time.Second)})
					case "obit_graveyard":
						st.Burials = append(st.Burials, ledger.Burial{ID: id, Path: path, Grave: filepath.Join(svc.Store.Home, "graveyard", fmt.Sprint(id), "candidate.log")})
					case "obit_proposals":
						st.Proposals = append(st.Proposals, ledger.Proposal{ID: id, Project: repo, Candidates: []ledger.Candidate{{Path: path, Lifespan: "1d"}}})
					}
				}
				st.NextID = 206
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				offset, limit, count int
				more                 bool
			}{{0, 0, 200, true}, {200, 10, 5, false}, {205, 1, 0, false}, {206, 1, 0, false}, {0, 1, 1, true}} {
				var data map[string]json.RawMessage
				clitest.CallMCP(t, s, tool, map[string]any{"after": fmt.Sprint(tc.offset), "limit": tc.limit}, false, &data)
				var rows []struct{ ID int64 }
				if err := json.Unmarshal(data[key], &rows); err != nil {
					t.Fatal(err)
				}
				var more bool
				var next string
				if err := json.Unmarshal(data["hasMore"], &more); err != nil {
					t.Fatalf("list lacks pagination metadata: %s", data)
				}
				if data["nextAfter"] != nil {
					if err := json.Unmarshal(data["nextAfter"], &next); err != nil {
						t.Fatal(err)
					}
				}
				wantNext := ""
				if tc.more {
					wantNext = fmt.Sprint(tc.offset + tc.count)
				}
				if len(rows) != tc.count || more != tc.more || next != wantNext {
					t.Fatalf("incorrect list page: rows=%d more=%v next=%s", len(rows), more, next)
				}
				for i, row := range rows {
					if row.ID != int64(tc.offset+i+1) {
						t.Fatalf("list page skipped or duplicated row: %+v", rows)
					}
				}
			}
			for _, args := range []map[string]any{{"limit": 201}, {"limit": -1}, {"after": "-1"}, {"after": "invalid"}, {"offset": 1}} {
				clitest.CallMCP(t, s, tool, args, true, nil)
			}
		})
	}
}

func TestMCPGraveyardCursorSurvivesChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, change := range []string{"restore-read", "restore-unread", "add"} {
		t.Run(change, func(t *testing.T) {
			s, repo, _ := mcpFixture(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 4; i++ {
				path := filepath.Join(repo, fmt.Sprintf("%d.log", i))
				clitest.WriteFixture(t, path, "buried")
				got, err := svc.Bury([]string{path}, false, "")
				if err != nil || core.GraveFailed(got) {
					t.Fatalf("bury: %+v %v", got, err)
				}
			}
			var page struct {
				Burials   []ledger.Burial
				NextAfter string
				HasMore   bool
			}
			clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"limit": 2}, false, &page)
			if len(page.Burials) != 2 || page.NextAfter != "2" {
				t.Fatalf("first page: %+v", page)
			}
			want := []int64{3, 4}
			if change == "add" {
				path := filepath.Join(repo, "5.log")
				clitest.WriteFixture(t, path, "new")
				got, err := svc.Bury([]string{path}, false, "")
				if err != nil || core.GraveFailed(got) {
					t.Fatal(got, err)
				}
				want = append(want, 5)
			} else {
				ref := "1"
				if change == "restore-unread" {
					ref, want = "3", []int64{4}
				}
				got, err := svc.Resurrect([]string{ref})
				if err != nil || core.GraveFailed(got) {
					t.Fatal(got, err)
				}
			}
			after := page.NextAfter
			var ids []int64
			for range 5 {
				page = struct {
					Burials   []ledger.Burial
					NextAfter string
					HasMore   bool
				}{}
				clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"limit": 1, "after": after}, false, &page)
				for _, b := range page.Burials {
					ids = append(ids, b.ID)
				}
				if !page.HasMore {
					break
				}
				after = page.NextAfter
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("cursor skipped surviving burials: got %v want %v", ids, want)
			}
		})
	}
}

func TestMCPGraveyardMergesPendingByID(t *testing.T) {
	svc, _, pending := clitest.PendingCommandBurial(t, false)
	if pending.Burial == nil {
		t.Fatal("pending fixture lacks burial")
	}
	b := *pending.Burial
	b.ID++
	if err := svc.Store.Update(func(st *ledger.State) error { st.Burials = append(st.Burials, b); st.NextID = b.ID + 1; return nil }); err != nil {
		t.Fatal(err)
	}
	s := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
	var page struct {
		Burials   []ledger.Burial
		Pending   []core.PendingGrave
		NextAfter string
		HasMore   bool
	}
	clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"limit": 1}, false, &page)
	if len(page.Pending) != 1 || len(page.Burials) != 0 || page.NextAfter != fmt.Sprint(pending.Burial.ID) {
		t.Fatalf("pending IDs were not merged before pagination: %+v", page)
	}
	after := page.NextAfter
	page = struct {
		Burials   []ledger.Burial
		Pending   []core.PendingGrave
		NextAfter string
		HasMore   bool
	}{}
	clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"limit": 1, "after": after}, false, &page)
	if len(page.Pending) != 0 || len(page.Burials) != 1 || page.Burials[0].ID != b.ID || page.HasMore || page.NextAfter != "" {
		t.Fatalf("combined cursor lost burial: %+v", page)
	}
}
