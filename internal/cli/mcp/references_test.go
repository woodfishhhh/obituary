package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	graveyardcmd "github.com/woodfishhhh/obituary/internal/cli/graveyard"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestMCPUnknownIDsNeverResolveAgainstCWD(t *testing.T) {
	for _, tool := range []string{"obit_spare", "obit_extend", "obit_resurrect"} {
		t.Run(tool, func(t *testing.T) {
			s, repo, _ := mcpFixture(t)
			for _, name := range []string{"999", "#999"} {
				path := filepath.Join(repo, name)
				clitest.WriteFixture(t, path, "user data")
				clitest.CallMCP(t, s, "obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1h"}, false, nil)
				if tool == "obit_resurrect" {
					clitest.CallMCP(t, s, "obit_bury", map[string]any{"paths": []string{path}}, false, nil)
				}
			}
			args := map[string]any{"refs": []string{"999"}}
			if tool == "obit_extend" {
				args["lifespan"] = "1h"
			}
			clitest.CallMCP(t, s, tool, args, true, nil)
			if tool == "obit_resurrect" {
				var graves graveyardcmd.GraveyardListing
				clitest.CallMCP(t, s, "obit_graveyard", struct{}{}, false, &graves)
				if len(graves.Burials) != 2 {
					t.Fatal("unknown ID resurrected a relative filename")
				}
			} else {
				var list struct{ Items []ledger.Item }
				clitest.CallMCP(t, s, "obit_list", struct{}{}, false, &list)
				if len(list.Items) != 2 {
					t.Fatal("unknown ID changed a relative filename")
				}
			}
		})
	}
}

func TestMCPBurialIDsClassifyBeforeParsing(t *testing.T) {
	t.Parallel()
	for _, ref := range []string{"9223372036854775808", "#99999999999999999999", "-9223372036854775809", " 9001", "9001 "} {
		if err := mcpReferences([]string{ref}, true, true); err == nil || !strings.Contains(err.Error(), "not a valid burial ID") || !strings.Contains(err.Error(), "filename use ./") {
			t.Fatalf("MCP numeric shape lost ID advice: %q %v", ref, err)
		}
	}
	for _, ref := range []string{"0", "-1", "007", "+5", "9223372036854775807"} {
		if err := mcpReferences([]string{ref}, true, true); err != nil {
			t.Fatalf("MCP rejected numeric burial reference: %q %v", ref, err)
		}
	}
}

func TestMCPRejectsRelativePaths(t *testing.T) {
	s, repo, starts := mcpFixture(t)
	clitest.WriteFixture(t, filepath.Join(repo, "relative.log"), "untouched")
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"obit_mark", map[string]any{"paths": []string{"relative.log"}, "lifespan": "1h"}},
		{"obit_bury", map[string]any{"paths": []string{"relative.log"}}},
		{"obit_resurrect", map[string]any{"refs": []string{"relative.log"}}},
		{"obit_extend", map[string]any{"refs": []string{"relative.log"}, "lifespan": "1h"}},
		{"obit_spare", map[string]any{"refs": []string{"relative.log"}}},
		{"obit_mktemp", map[string]any{"cwd": ".", "lifespan": "1h"}},
		{"obit_bury", map[string]any{"paths": []string{filepath.Join(repo, "relative.log"), "C:relative.log"}}},
	} {
		r := clitest.CallMCP(t, s, tc.tool, tc.args, true, nil)
		b, _ := json.Marshal(r.Content)
		if !strings.Contains(string(b), "absolute") {
			t.Fatalf("missing absolute-path explanation: %s", b)
		}
	}
	if b, err := os.ReadFile(filepath.Join(repo, "relative.log")); err != nil || string(b) != "untouched" || len(*starts) != 0 {
		t.Fatalf("rejected request changed file/daemon: %q %v %v", b, err, *starts)
	}
}

func TestMCPNumericShapedReferencePreservesGrave(t *testing.T) {
	for _, ref := range []string{"9223372036854775808", "#99999999999999999999", "-9223372036854775809", " 9001", "9001 "} {
		t.Run(ref, func(t *testing.T) {
			session, repo, _ := mcpFixture(t)
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repo, ref)
			clitest.WriteFixture(t, path, "keep")
			clitest.CallMCP(t, session, "obit_bury", map[string]any{"paths": []string{path}}, false, nil)
			clitest.CallMCP(t, session, "obit_resurrect", map[string]any{"refs": []string{ref}}, true, nil)
			st, err := svc.Store.ReadSnapshot()
			if err != nil || len(st.Burials) != 1 {
				t.Fatalf("MCP numeric shape changed graveyard: %+v %v", st, err)
			}
			data, err := os.ReadFile(st.Burials[0].Grave)
			if err != nil || string(data) != "keep" {
				t.Fatal("MCP numeric shape lost grave content")
			}
		})
	}
}
