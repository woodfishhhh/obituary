//go:build windows

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	graveyardcmd "github.com/woodfishhhh/obituary/internal/cli/graveyard"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
)

func TestMCPContainerLists(t *testing.T) {
	clitest.IsolateRunner(t)
	clitest.CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	id := docker.Identity{ID: strings.Repeat("a", 64), ShortID: strings.Repeat("a", 12), Name: "cache", Created: now}
	if err := svc.Store.Update(func(st *ledger.State) error {
		st.Items = []ledger.Item{{ID: 1, Rev: 1, Kind: ledger.KindContainer, Container: &id, Path: id.Display(), ExpiresAt: now.Add(time.Hour)}}
		st.Burials = []ledger.Burial{{ID: 2, Kind: ledger.KindContainer, Container: &id, Path: id.Display(), BuriedAt: now, PurgeAt: now.Add(time.Hour)}}
		st.Proposals = []ledger.Proposal{{ID: 3, Kind: core.ProposalContainer, Candidates: []ledger.Candidate{{Kind: ledger.KindContainer, Container: &id, Path: id.Display(), Lifespan: "2d"}}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for name, reply := range map[string]mcpReply{"list": listMCP(svc, mcpPageInput{}), "graveyard": graveyardMCP(svc, mcpGraveyardInput{}), "proposals": proposalsMCP(svc, mcpPageInput{})} {
		data, err := json.Marshal(reply)
		if err != nil || !strings.Contains(string(data), "docker:cache") || !strings.Contains(string(data), `"kind":"container"`) || !strings.Contains(string(data), `"shortId"`) {
			t.Fatalf("container absent from %s: %s %v", name, data, err)
		}
	}
}

func TestMCPContainerOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	clitest.IsolateRunner(t)
	clitest.CommandDir(t)
	executable, state, _ := testenv.FakeDocker(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	svc.Now = func() time.Time { return now }
	svc.Docker = docker.CLI{Executable: executable}
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("a", 64), Name: "cache", Created: now.Add(-time.Hour)}, Running: true}
	testenv.WriteDocker(t, state, row)
	it, err := svc.AddContainer("cache", core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	session := clitest.ConnectMCP(t, NewServer(svc, "test"))
	var items struct{ Results []core.Result }
	clitest.CallMCP(t, session, "obit_extend", map[string]any{"refs": []string{fmt.Sprint(it.ID)}, "lifespan": "1h"}, false, &items)
	if len(items.Results) != 1 || items.Results[0].Item.Kind != ledger.KindContainer {
		t.Fatal("container extend lost receipt", items)
	}
	now = now.Add(3 * time.Hour)
	if _, err := svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	var graves struct{ Results []core.GraveResult }
	clitest.CallMCP(t, session, "obit_resurrect", map[string]any{"refs": []string{fmt.Sprint(it.ID)}}, false, &graves)
	if len(graves.Results) != 1 || graves.Results[0].Action != core.ActionResurrected {
		t.Fatal("container resurrection lost", graves)
	}
	it, err = svc.AddContainer("cache", core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	clitest.CallMCP(t, session, "obit_spare", map[string]any{"refs": []string{fmt.Sprint(it.ID)}}, false, &items)
	if len(items.Results) != 1 || items.Results[0].Action != core.ActionCanceled {
		t.Fatal("container spare lost", items)
	}
}

func TestMCPRecycleModeDescriptionAndReceipt(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[graveyard]\nmode = 'recycle'\n")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	if svc.Mode != core.ModeRecycle {
		t.Fatal("fixture did not load recycle mode")
	}
	destination := filepath.Join(t.TempDir(), "recycled.log")
	svc.FS = mcpRecycleFS{svc.FS, destination}
	s := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
	tools, err := s.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "obit_bury" && tool.Description != "Moves absolute paths to the system Recycle Bin immediately, with the same Guard and identity checks as obit bury. Restore from the Recycle Bin; obit_resurrect cannot restore these files." {
			t.Errorf("recycle description promises wrong recovery: %s", tool.Description)
		}
	}
	path := filepath.Join(repo, "recycle.log")
	clitest.WriteFixture(t, path, "safe fixture")
	var out struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": []string{path}}, false, &out)
	if len(out.Results) != 1 || out.Results[0].Action != core.ActionRecycled || out.Results[0].Burial != nil {
		t.Fatalf("recycle receipt inaccurate: %+v", out)
	}
	if !strings.Contains(out.Results[0].Detail, "restore from the Recycle Bin") || strings.Contains(out.Results[0].Detail, "obit resurrect") {
		t.Fatalf("recycle receipt promises wrong recovery: %+v", out)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "safe fixture" {
		t.Fatalf("fake recycle not used: %q %v", got, err)
	}
	clitest.CallMCP(t, s, "obit_resurrect", map[string]any{"refs": []string{path}}, true, nil)
}

func TestMCPToolInventory(t *testing.T) {
	descriptions := map[string]string{
		"obit_mark":      "Registers absolute paths with a lifespan, as obit add does. Re-registering resets expiry. Guard and identity checks apply.",
		"obit_mktemp":    "Creates and registers a temporary directory with a lifespan. The explicit absolute cwd selects the Git project; outside Git the system temporary directory is used.",
		"obit_bury":      "Moves absolute paths into the restorable graveyard immediately, with the same Guard and identity checks as obit bury.",
		"obit_resurrect": "Restores verified burials by ID or absolute original path. Existing files are never overwritten; the result reports the restored path.",
		"obit_extend":    "Extends tracked lifespans by ID or absolute path, counted from now when already due.",
		"obit_spare":     "Stops tracking IDs or absolute paths and leaves files in place.",
		"obit_list":      "Lists tracked items in ascending ID order; pass nextAfter as after and version to continue. changed warns when the full set changes; restart for a complete view. total counts this read.",
		"obit_graveyard": "Lists burials and pending manifests together in ascending ID order; reconciles verified interrupted restores and reports their actual paths. Optional here filters original paths by an absolute directory, including descendants. Pass nextAfter as after and version to continue with the same here. changed warns when the full set changes; restart for a complete view. total counts this read. Permanent purging is a user decision.",
		"obit_proposals": "Lists proposals in ascending ID order; pass nextAfter as after and version to continue. changed warns when the full set changes; restart for a complete view. total counts this read. Approval and rejection belong to the user, who executes obit approve or obit reject; this server cannot decide them.",
		"obit_stats":     "Reports tracked items, graveyard size, lifetime counters and delete guard counts.",
	}
	s, _, _ := mcpFixture(t)
	r, err := s.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range r.Tools {
		names = append(names, tool.Name)
		if want, known := descriptions[tool.Name]; known && tool.Description != want {
			t.Fatalf("tool description changed for %s: %q", tool.Name, tool.Description)
		}
		if tool.InputSchema == nil || tool.OutputSchema != nil || tool.Description == "" {
			t.Fatalf("missing schema or description: %+v", tool)
		}
		if tool.Name == "obit_proposals" && !strings.Contains(tool.Description, "user") {
			t.Fatal("approval ownership omitted")
		}
		if tool.Name == "obit_graveyard" {
			data, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				Properties map[string]struct{ Type string }
				Required   []string
			}
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatal(err)
			}
			if schema.Properties["here"].Type != "string" || len(schema.Properties) != 4 || len(schema.Required) != 0 {
				t.Fatalf("graveyard schema must expose optional here and pagination: %s", data)
			}
		}
	}
	sort.Strings(names)
	want := []string{"obit_bury", "obit_extend", "obit_graveyard", "obit_list", "obit_mark", "obit_mktemp", "obit_proposals", "obit_resurrect", "obit_spare", "obit_stats"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("tool inventory = %v, want %v (approve/purge and other user decisions must stay absent)", names, want)
	}
}

func TestMCPGraveyardHereAndBurialBatches(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	paths := []string{filepath.Join(repo, "a", "first"), filepath.Join(repo, "a", "sub", "second"), filepath.Join(repo, "ab", "outside")}
	for _, path := range paths {
		clitest.WriteFixture(t, path, "keep")
	}
	var batch struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": paths[:2]}, false, &batch)
	if len(batch.Results) != 2 || batch.Results[0].Burial.Operation == nil || batch.Results[1].Burial.Operation == nil || batch.Results[0].Burial.Operation.ID != batch.Results[1].Burial.Operation.ID {
		t.Fatalf("MCP call split batch: %+v", batch)
	}
	firstID := batch.Results[0].Burial.Operation.ID
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	refs, _, err := svc.LastBurialRefs("", repo)
	if err != nil || len(refs) != 2 || refs[0] != fmt.Sprint(batch.Results[0].Burial.ID) || refs[1] != fmt.Sprint(batch.Results[1].Burial.ID) {
		t.Fatalf("same-project MCP batch unavailable to CLI --last: %v %v", refs, err)
	}
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": paths[2:]}, false, &batch)
	if len(batch.Results) != 1 || batch.Results[0].Burial.Operation == nil || batch.Results[0].Burial.Operation.ID == firstID {
		t.Fatal("MCP calls shared a deletion batch")
	}
	var listing struct {
		Burials            []ledger.Burial
		Total              int
		NextAfter, Version string
		HasMore            bool
	}
	clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"here": filepath.Join(repo, "a"), "limit": 1}, false, &listing)
	if listing.Total != 2 || len(listing.Burials) != 1 || !listing.HasMore || listing.Burials[0].Path != paths[0] {
		t.Fatalf("filter after pagination: %+v", listing)
	}
	clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"here": filepath.Join(repo, "a"), "limit": 1, "after": listing.NextAfter, "version": listing.Version}, false, &listing)
	if listing.Total != 2 || len(listing.Burials) != 1 || listing.HasMore || listing.Burials[0].Path != paths[1] {
		t.Fatalf("filter lost pagination boundary: %+v", listing)
	}
	clitest.CallMCP(t, s, "obit_graveyard", map[string]any{"here": "a"}, true, nil)
}

func TestMCPMixedProjectBatchHasNoInferredScope(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	paths := []string{filepath.Join(repo, "inside"), filepath.Join(t.TempDir(), "outside")}
	for _, path := range paths {
		clitest.WriteFixture(t, path, "keep")
	}
	var batch struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": paths}, false, &batch)
	if len(batch.Results) != 2 {
		t.Fatal(batch)
	}
	for _, row := range batch.Results {
		if row.Burial.Operation == nil || row.Burial.Operation.Project != "" || row.Burial.Operation.Session != "" {
			t.Fatalf("MCP borrowed the server scope: %+v", row.Burial)
		}
	}
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	if refs, _, err := svc.LastBurialRefs("", repo); err == nil {
		t.Fatalf("mixed-project batch guessed: %v", refs)
	}
}

func TestMCPFileLifecycle(t *testing.T) {
	s, repo, starts := mcpFixture(t)
	path := filepath.Join(repo, "debug.txt")
	clitest.WriteFixture(t, path, "retained bytes")
	var items struct{ Results []core.Result }
	clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{path}, "lifespan": "2h"}, false, &items)
	if len(items.Results) != 1 || items.Results[0].Item == nil || len(*starts) != 1 {
		t.Fatalf("mark receipt/daemon: %+v %v", items, *starts)
	}
	item := items.Results[0].Item
	ref := fmt.Sprint(item.ID)
	expires := item.ExpiresAt
	clitest.CallMCP(t, s, "obit_extend", map[string]any{"refs": []string{ref}, "lifespan": "1h"}, false, &items)
	if !items.Results[0].Item.ExpiresAt.Equal(expires.Add(time.Hour)) {
		t.Fatal("extend did not add one hour")
	}
	var graves struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": []string{path}}, false, &graves)
	if _, err := os.Stat(path); !os.IsNotExist(err) || len(*starts) != 2 {
		t.Fatalf("bury did not move/start: %v %v", err, *starts)
	}
	var listing graveyardcmd.GraveyardListing
	clitest.CallMCP(t, s, "obit_graveyard", struct{}{}, false, &listing)
	if len(listing.Burials) != 1 || listing.Burials[0].ID != item.ID {
		t.Fatalf("graveyard: %+v", listing)
	}
	clitest.CallMCP(t, s, "obit_resurrect", map[string]any{"refs": []string{ref}}, false, &graves)
	if got, err := os.ReadFile(graves.Results[0].Path); err != nil || string(got) != "retained bytes" {
		t.Fatalf("round trip: %q %v", got, err)
	}
	clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1h"}, false, nil)
	clitest.CallMCP(t, s, "obit_spare", map[string]any{"refs": []string{path}}, false, nil)
	var list struct{ Items []ledger.Item }
	clitest.CallMCP(t, s, "obit_list", struct{}{}, false, &list)
	if len(list.Items) != 0 {
		t.Fatal("spare left tracked items")
	}
	var summary core.Summary
	clitest.CallMCP(t, s, "obit_stats", struct{}{}, false, &summary)
	if summary.Lifetime.Buried != 1 || summary.Lifetime.Resurrected != 1 {
		t.Fatalf("stats: %+v", summary)
	}
}

func TestMCPGuardErrorsLeaveServerUsable(t *testing.T) {
	s, repo, starts := mcpFixture(t)
	for _, name := range []string{"obit_mark", "obit_bury"} {
		args := map[string]any{"paths": []string{repo}}
		if name == "obit_mark" {
			args["lifespan"] = "1h"
		}
		clitest.CallMCP(t, s, name, args, true, nil)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil || len(*starts) != 0 {
		t.Fatalf("Guard moved project: %v %v", err, *starts)
	}
	clitest.CallMCP(t, s, "obit_proposals", struct{}{}, false, nil)
	clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{repo}, "lifespan": "invalid"}, true, nil)
	clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{}}, true, nil)
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": "wrong type"}, true, nil)
	clitest.CallMCP(t, s, "obit_list", struct{}{}, false, nil)
}

func TestMCPMktempUsesExplicitProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	s, repo, starts := mcpGitFixture(t)
	t.Chdir(t.TempDir())
	var result struct {
		Path string
		Item ledger.Item
	}
	clitest.CallMCP(t, s, "obit_mktemp", map[string]any{"cwd": repo, "lifespan": "1d", "name": "scratch", "session": "mcp-test"}, false, &result)
	if !strings.HasPrefix(result.Path, filepath.Join(repo, ".obit", "tmp")) || result.Item.Source != "mktemp" || result.Item.Session != "mcp-test" || len(*starts) != 1 {
		t.Fatalf("mktemp: %+v %v", result, *starts)
	}
}

func TestMCPResurrectPrefersLatestCommittedBurial(t *testing.T) {
	svc, older, newer := clitest.CommittedRetryFixture(t)
	session := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
	var reply struct{ Results []core.GraveResult }
	clitest.CallMCP(t, session, "obit_resurrect", map[string]any{"refs": []string{newer.Path}}, false, &reply)
	clitest.AssertCommittedRetry(t, svc, older, newer, reply.Results)
}

func TestMCPResurrectRetriesUnverifiableSelection(t *testing.T) {
	for _, mode := range []string{"manifest", "scan"} {
		t.Run(mode, func(t *testing.T) {
			svc, older, newer := clitest.BurialRetryFixture(t, mode)
			session := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
			result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "obit_resurrect", Arguments: map[string]any{"refs": []string{newer.Path}}})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			clitest.AssertRetryPreservesBurials(t, older, newer, mode, string(data), result.IsError)
		})
	}
}

func TestMCPResurrectExpandsAbsoluteShortPath(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	path := filepath.Join(repo, "x.txt")
	clitest.WriteFixture(t, path, "keep")
	p, _ := windows.UTF16PtrFromString(path)
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(buf[:n])
	if !strings.Contains(short, "~") {
		t.Skip("volume has no short names")
	}
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": []string{path}}, false, nil)
	var reply struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_resurrect", map[string]any{"refs": []string{short}}, false, &reply)
	if len(reply.Results) != 1 || reply.Results[0].Path != path {
		t.Fatalf("MCP short reference lost original path: %+v", reply)
	}
}

func TestMCPResurrectReportsPartialCommit(t *testing.T) {
	svc, paths, unlock := clitest.RestoreReceiptFixture(t)
	s := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
	var data struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_resurrect", map[string]any{"refs": paths}, true, &data)
	actual := filepath.Join(filepath.Dir(paths[1]), "b (resurrected).txt")
	if len(data.Results) != 3 || data.Results[0].Path != paths[0] || data.Results[1].Path != actual || data.Results[1].Error == "" || data.Results[2].Action != core.ActionNotExecuted {
		t.Fatalf("MCP restore partial receipt lost completed paths: %+v", data)
	}
	unlock()
	var view struct {
		Burials  []ledger.Burial
		Restored []core.GraveResult
	}
	clitest.CallMCP(t, s, "obit_graveyard", map[string]any{}, false, &view)
	if len(view.Burials) != 1 || view.Burials[0].Path != paths[2] || len(view.Restored) != 1 || view.Restored[0].Path != actual {
		t.Fatalf("MCP graveyard lost recovered restore path: %+v", view)
	}
}

func TestMCPGraveyardIncludesConcurrentRecovery(t *testing.T) {
	svc, _, pending := clitest.PendingCommandBurial(t, false)
	finish := clitest.RecoverDuringGraveyardRead(t, svc)
	s := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
	for range 2 {
		var page struct {
			Burials []ledger.Burial
			Pending []core.PendingGrave
		}
		clitest.CallMCP(t, s, "obit_graveyard", map[string]any{}, false, &page)
		if len(page.Burials)+len(page.Pending) != 1 || len(page.Burials) == 1 && page.Burials[0].ID != pending.Burial.ID || len(page.Pending) == 1 && page.Pending[0].Burial.ID != pending.Burial.ID {
			t.Fatalf("graveyard view lost burial during recovery: %+v", page)
		}
		if finish != nil {
			finish()
			finish = nil
		}
	}
}

func TestMCPProposalsOmitsMissingCandidates(t *testing.T) {
	root, store := clitest.CommandDir(t)
	keep, gone := filepath.Join(root, "keep.txt"), filepath.Join(root, "gone.txt")
	clitest.WriteFixture(t, keep, "keep")
	clitest.AddProposal(t, store, root, ledger.Candidate{Path: keep, Lifespan: "1d"}, ledger.Candidate{Path: gone, Lifespan: "1d"})
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	session := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
	t.Setenv("PATH", t.TempDir())
	var data struct{ Proposals []ledger.Proposal }
	clitest.CallMCP(t, session, "obit_proposals", struct{}{}, false, &data)
	if len(data.Proposals) != 1 || len(data.Proposals[0].Candidates) != 1 || data.Proposals[0].Candidates[0].Path != keep {
		t.Fatalf("MCP displayed missing candidates: %+v", data)
	}
	if err := os.Remove(keep); err != nil {
		t.Fatal(err)
	}
	clitest.CallMCP(t, session, "obit_proposals", struct{}{}, false, &data)
	if len(data.Proposals) != 0 {
		t.Fatalf("MCP displayed empty proposal: %+v", data)
	}
}

func TestMCPMkTempReportsFailedRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	starts := clitest.IsolateRunner(t)
	repo, store := clitest.CommandRepo(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	svc.FS = cancelMCPTempFS{FS: svc.FS, cancel: cancel}
	server := NewServer(svc, "0.1.0-dev")
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(request context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "tools/call" {
				request = ctx
			}
			return next(request, method, req)
		}
	})
	var data struct {
		Path string
		Item *ledger.Item
	}
	r := clitest.CallMCP(t, clitest.ConnectMCP(t, server), "obit_mktemp", map[string]any{"cwd": repo, "lifespan": "1d", "name": "rollback"}, true, &data)
	encoded, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if data.Path == "" || data.Item != nil || !strings.Contains(string(encoded), "rollback temporary directory") {
		t.Fatalf("MCP lost rollback path: %s", encoded)
	}
	if _, err := os.Stat(filepath.Join(data.Path, "keep.txt")); err != nil {
		t.Fatal(err)
	}
	st, err := store.Inspect()
	if err != nil || len(st.Items) != 0 || len(*starts) != 0 {
		t.Fatalf("failed mktemp committed or started runner: %+v %v %v", st, *starts, err)
	}
}

func TestMCPRejectsOversizedMetadataAndBatches(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	path := filepath.Join(repo, "note.txt")
	clitest.WriteFixture(t, path, "keep")
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	many := make([]string, 101)
	for i := range many {
		many[i] = path
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1h", "note": strings.Repeat("n", 9<<20)}},
		{"obit_bury", map[string]any{"paths": []string{path}, "note": strings.Repeat("n", 4097)}},
		{"obit_mktemp", map[string]any{"cwd": repo, "lifespan": "1h", "session": strings.Repeat("s", 4097)}},
		{"obit_mark", map[string]any{"paths": many, "lifespan": "1h"}},
		{"obit_bury", map[string]any{"paths": many}},
		{"obit_spare", map[string]any{"refs": many}},
		{"obit_extend", map[string]any{"refs": many, "lifespan": "1h"}},
		{"obit_resurrect", map[string]any{"refs": many}},
	} {
		r, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil || !r.IsError {
			t.Fatalf("oversized MCP input accepted: %s, transport=%v", tc.name, err)
		}
		after, err := svc.Store.Read()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("oversized MCP input mutated ledger: %s %v", tc.name, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMCPListsLegacyLongNotes(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "note.txt")
	clitest.WriteFixture(t, path, "keep")
	if err := svc.Store.Update(func(st *ledger.State) error {
		st.Items = []ledger.Item{{ID: 1, Path: path, Note: strings.Repeat("n", 9<<20), ExpiresAt: time.Now().Add(time.Hour)}}
		st.NextID = 2
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: "obit_list", Arguments: map[string]any{}})
	if err != nil || r.IsError {
		t.Fatalf("legacy note blocked listing: %v", err)
	}
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) >= 1<<20 || !strings.Contains(string(encoded), "[truncated]") {
		t.Fatalf("legacy note exceeded bounded receipt: bytes=%d err=%v", len(encoded), err)
	}
	after, err := svc.Store.Read()
	if err != nil || len(after.Items[0].Note) != 9<<20 {
		t.Fatalf("display truncation changed ledger: %v", err)
	}
}
