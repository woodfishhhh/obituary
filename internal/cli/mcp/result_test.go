package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestMCPPartialBurialKeepsReceipts(t *testing.T) {
	s, repo, starts := mcpFixture(t)
	path := filepath.Join(repo, "partial.log")
	clitest.WriteFixture(t, path, "recoverable")
	var result struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": []string{path, repo}}, true, &result)
	if len(result.Results) != 2 || result.Results[0].Action != core.ActionBuried || result.Results[1].Error == "" || len(*starts) != 1 {
		t.Fatalf("partial receipts/daemon lost: %+v %v", result, *starts)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
		t.Fatal("Guard did not preserve project", err)
	}
}

func TestMCPResurrectPreflightAuditFailureIsError(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	path := filepath.Join(repo, "restore.log")
	clitest.WriteFixture(t, path, "still buried")
	var graves struct{ Results []core.GraveResult }
	clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": []string{path}}, false, &graves)
	burial := graves.Results[0].Burial
	other := filepath.Join(repo, "duplicate.log")
	clitest.WriteFixture(t, other, "legacy duplicate")
	var marked struct{ Results []core.Result }
	clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{other}, "lifespan": "1d"}, false, &marked)
	store := core.Store{Home: os.Getenv("OBIT_HOME")}
	if err := store.UpdateHot(func(st *core.State) error {
		duplicate := st.Items[0]
		duplicate.ID = st.NextID
		st.NextID++
		st.Items = append(st.Items, duplicate)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// A directory at the audit path forces the migration audit to fail.
	if err := os.Rename(filepath.Join(store.Home, "audit.jsonl"), filepath.Join(store.Home, "audit.saved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(store.Home, "audit.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	r, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: "obit_resurrect", Arguments: map[string]any{"refs": []string{fmt.Sprint(burial.ID)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("preflight unexpectedly restored file: %v", err)
	}
	if got, err := os.ReadFile(burial.Grave); err != nil || string(got) != "still buried" {
		t.Fatalf("burial changed: %q %v", got, err)
	}
	if !r.IsError {
		t.Fatalf("preflight audit failure reported success: %+v", r)
	}
	b, _ := json.Marshal(r.StructuredContent)
	var reply mcpReply
	if err := json.Unmarshal(b, &reply); err != nil || reply.Error == "" || reply.Warning != "" {
		t.Fatalf("preflight must be an error: %s %v", b, err)
	}
	if !strings.Contains(reply.Error, "audit") {
		t.Fatalf("preflight fixture did not reach audit delivery: %s", b)
	}
}

func TestMCPDaemonFailureKeepsReceipts(t *testing.T) {
	for _, tool := range []string{"obit_mark", "obit_bury", "obit_mktemp"} {
		t.Run(tool, func(t *testing.T) {
			if testing.Short() && tool == "obit_mktemp" {
				t.Skip("mktemp updates the real Git exclusion file")
			}
			s, repo, _ := mcpFixture(t)
			if tool == "obit_mktemp" {
				testenv.InitRepo(t, repo)
			}
			cmdutil.RunnerStart = func(string) error { return errors.New("start denied") }
			path := filepath.Join(repo, "daemon.log")
			clitest.WriteFixture(t, path, "receipt")
			args := map[string]any{"paths": []string{path}}
			if tool == "obit_mark" {
				args["lifespan"] = "1d"
			}
			if tool == "obit_mktemp" {
				args = map[string]any{"cwd": repo, "lifespan": "1d"}
			}
			r := clitest.CallMCP(t, s, tool, args, false, nil)
			b, _ := json.Marshal(r.StructuredContent)
			var reply mcpReply
			if err := json.Unmarshal(b, &reply); err != nil || reply.Data == nil || !strings.Contains(reply.Warning, "obit daemon enable") || !strings.Contains(reply.Warning, "start denied") {
				t.Fatalf("daemon failure warning or receipt missing: %s %v", b, err)
			}
		})
	}
}

func TestMCPAuditWarningKeepsCommittedResult(t *testing.T) {
	s, repo, starts := mcpFixture(t)
	path := filepath.Join(repo, "audit.log")
	clitest.WriteFixture(t, path, "retained")
	if err := os.MkdirAll(filepath.Join(os.Getenv("OBIT_HOME"), "audit.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	var result struct{ Results []core.Result }
	r := clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1d"}, false, &result)
	data, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var reply mcpReply
	if err := json.Unmarshal(data, &reply); err != nil || reply.Warning == "" || len(result.Results) != 1 || result.Results[0].Item == nil || len(*starts) != 1 {
		t.Fatalf("committed result not reported: %s %v", data, err)
	}
	var list struct{ Items []core.Item }
	clitest.CallMCP(t, s, "obit_list", struct{}{}, false, &list)
	if len(list.Items) != 1 {
		t.Fatal("audit warning hid registration")
	}
}
