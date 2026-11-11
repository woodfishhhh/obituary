package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestMCPReplyTotalBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	rows := make([]core.GraveResult, 9000)
	for i := range rows {
		rows[i] = core.GraveResult{Ref: "1", Action: core.ActionBuried, Burial: &core.Burial{ID: int64(i + 1), Path: strings.Repeat("<", 4096), Note: strings.Repeat("n", 4096)}}
	}
	out := boundMCPReply(mcpResult(map[string]any{"results": rows}, nil))
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := json.Marshal(mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}, StructuredContent: json.RawMessage(data)})
	if err != nil || len(frame) > 1<<20 || !out.Truncated {
		t.Fatalf("MCP final reply exceeded total budget: bytes=%d err=%v", len(frame), err)
	}
}

func TestMCPRepeatedCompactionRetainsOriginalTotals(t *testing.T) {
	t.Parallel()
	proposals := make([]core.Proposal, 200)
	for i := range proposals {
		p := &proposals[i]
		p.ID = int64(i + 1)
		p.Project = strings.Repeat("p", 1000)
		p.Candidates = make([]core.Candidate, 100)
		for j := range p.Candidates {
			p.Candidates[j] = core.Candidate{Path: strings.Repeat("f", 1000), Rule: strings.Repeat("r", 1000), Lifespan: "1d"}
		}
	}
	reply := boundMCPReply(mcpResult(map[string]any{"proposals": proposals}, nil))
	encoded, err := json.Marshal(reply)
	var decoded struct {
		Data struct {
			Proposals []struct {
				CandidatesTotal int
				Candidates      []json.RawMessage
			}
		}
	}
	if err != nil || json.Unmarshal(encoded, &decoded) != nil || len(encoded) > mcpReplyBytes || len(decoded.Data.Proposals) != 200 {
		t.Fatalf("proposal budget: %d %v", len(encoded), err)
	}
	for i, p := range decoded.Data.Proposals {
		if p.CandidatesTotal != 100 || len(p.Candidates) >= 100 {
			t.Fatalf("second compaction replaced original candidate total: row=%d total=%d shown=%d", i, p.CandidatesTotal, len(p.Candidates))
		}
	}
}

func TestMCPUnconfirmedLongHomeKeepsAdvice(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	if os.Getenv("OBIT_TEST_MCP_SYNC_CHILD") != "1" {
		// The core failure seam is intentionally not a production API. Like
		// the mutation suite, inject it into a disposable compiled overlay;
		// the child still calls the real MCP server and all three handlers.
		root, err := filepath.Abs("../../..")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "internal", "core", "ledger", "store.go")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, []byte(`
func init() {
	fileio.SyncReplacement = func(f *os.File) error {
		if os.Getenv("OBIT_TEST_MCP_SYNC_FAIL") == "1" && filepath.Base(f.Name()) == "store.json.tmp" {
			return &os.PathError{Op: "sync", Path: f.Name(), Err: os.ErrPermission}
		}
		return f.Sync()
	}
}
`)...)
		dir := t.TempDir()
		replacement := filepath.Join(dir, "store.go")
		if err := os.WriteFile(replacement, data, 0600); err != nil {
			t.Fatal(err)
		}
		overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{path: replacement}})
		if err != nil {
			t.Fatal(err)
		}
		overlayPath := filepath.Join(dir, "overlay.json")
		if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
			t.Fatal(err)
		}
		// The child rebuilds every package above core with the overlay; slower CI
		// runners need more than a minute just to compile it.
		ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlayPath, "-count=1", "-run", "^TestMCPUnconfirmedLongHomeKeepsAdvice$", "./internal/cli/mcp")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "OBIT_TEST_MCP_SYNC_CHILD=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("MCP sync fault fixture failed: %v\n%s", err, out)
		}
		return
	}
	for _, action := range []string{"mark", "extend", "spare"} {
		t.Run(action, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			t.Setenv("OBIT_HOME", filepath.Join(store.Home, strings.Repeat("长目录", 55)))
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			session := clitest.ConnectMCP(t, NewServer(svc, "0.1.0-dev"))
			path := filepath.Join(repo, "payload.txt")
			clitest.WriteFixture(t, path, "keep")
			clitest.CallMCP(t, session, "obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1h"}, false, nil)
			t.Setenv("OBIT_TEST_MCP_SYNC_FAIL", "1")
			args := map[string]any{"refs": []string{path}, "lifespan": "2h"}
			switch action {
			case "mark":
				args = map[string]any{"paths": []string{path}, "lifespan": "2h"}
			case "spare":
				delete(args, "lifespan")
			}
			r := clitest.CallMCP(t, session, "obit_"+action, args, true, nil)
			encoded, err := json.Marshal(r.StructuredContent)
			var reply mcpReply
			if err != nil || json.Unmarshal(encoded, &reply) != nil || !reply.Truncated || !utf8.ValidString(reply.Error) || !strings.Contains(reply.Error, "commit unconfirmed") || !strings.Contains(reply.Error, "may already have taken effect") || !strings.Contains(reply.Error, "check obit list before retrying") {
				t.Fatalf("MCP long Home displaced commit advice: %s %v", encoded, err)
			}
		})
	}
}

func TestMCPRepeatedExtendReceiptIsBounded(t *testing.T) {
	s, repo, _ := mcpFixture(t)
	path := filepath.Join(repo, "note.txt")
	clitest.WriteFixture(t, path, "keep")
	var marked struct{ Results []core.Result }
	clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1h", "note": strings.Repeat("n", 4096)}, false, &marked)
	refs := make([]string, 100)
	for i := range refs {
		refs[i] = path
	}
	var extended struct{ Results []core.Result }
	r := clitest.CallMCP(t, s, "obit_extend", map[string]any{"refs": refs, "lifespan": "1h"}, false, &extended)
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) >= 1<<20 || len(extended.Results) != 100 {
		t.Fatalf("repeated extend receipt too large or incomplete: %d bytes, %d rows, %v", len(encoded), len(extended.Results), err)
	}
	if extended.Results[99].Item.ID != marked.Results[0].Item.ID || !extended.Results[99].Item.ExpiresAt.Equal(marked.Results[0].Item.ExpiresAt.Add(100*time.Hour)) {
		t.Fatal("bounded receipt lost final identity or expiry")
	}
}

func TestMCPBoundsNestedLegacyData(t *testing.T) {
	t.Parallel()
	const largeID = int64(9007199254740993)
	var candidates []core.Candidate
	for range 200 {
		candidates = append(candidates, core.Candidate{Path: strings.Repeat("\x00", 32768), Rule: strings.Repeat("rule", 4096), Lifespan: "1d"})
	}
	p := core.Proposal{ID: largeID, Project: strings.Repeat("p", 32768), Session: strings.Repeat("s", 4096), Candidates: candidates}
	reply := boundMCPReply(mcpResult(map[string]any{"proposals": []core.Proposal{p}}, nil))
	encoded, err := json.Marshal(reply)
	if err != nil || len(encoded) > mcpRowBytes+1024 || !reply.Truncated || !strings.Contains(string(encoded), "candidatesTotal") || !strings.Contains(string(encoded), "9007199254740993") {
		t.Fatalf("nested legacy receipt not bounded or lost precision: bytes=%d error=%v", len(encoded), err)
	}
	if len(p.Candidates) != 200 || len(p.Candidates[0].Path) != 32768 {
		t.Fatal("display mutated original state")
	}
}
