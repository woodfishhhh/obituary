//go:build windows

package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/project"
)

// Exercise the installed command/args with the official SDK. This verifies the
// exported local MCP connection; consumer WorkBuddy UI/loader remains untested.
func TestWorkBuddyInstalledMCPLocalRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, git := range []bool{false, true} {
		name := "office"
		if git {
			name = "git"
		}
		t.Run(name, func(t *testing.T) {
			repo, store := commandRepo(t)
			dir := repo
			if !git {
				dir = filepath.Join(filepath.Dir(repo), "办公 files")
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if _, inRepo, err := project.Find(dir); err != nil || inRepo != git {
				t.Fatal("fixture Git status", inRepo, err)
			}
			binary := binaryPath(t, "obit")
			data, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			binary = filepath.Join(t.TempDir(), "工具 with spaces", "obit.exe")
			writeFixture(t, binary, string(data))
			scope := integrate.Scope{Project: dir}
			plan, err := integrate.PlanWorkBuddyMCP(scope, binary, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := integrate.Apply(plan); err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile(filepath.Join(dir, ".workbuddy", "mcp.json"))
			var installed struct {
				Servers map[string]struct {
					Command string   `json:"command"`
					Args    []string `json:"args"`
				} `json:"mcpServers"`
			}
			if err != nil || json.Unmarshal(config, &installed) != nil {
				t.Fatal("read installed MCP", err, string(config))
			}
			entry := installed.Servers["obituary"]
			cmd := exec.Command(entry.Command, entry.Args...)
			cmd.Dir = dir
			client := mcp.NewClient(&mcp.Implementation{Name: "workbuddy-config-test", Version: "1"}, nil)
			session, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: cmd}, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { session.Close() })
			path := filepath.Join(dir, "可恢复 sample.txt")
			writeFixture(t, path, "local WorkBuddy MCP round trip")
			var buried struct{ Results []core.GraveResult }
			clitest.CallMCP(t, session, "obit_bury", map[string]any{"paths": []string{path}}, false, &buried)
			if len(buried.Results) != 1 || buried.Results[0].Action != core.ActionBuried || buried.Results[0].Burial == nil {
				t.Fatal("burial receipt", buried)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("file not buried", err)
			}
			var graves struct{ Burials []core.Burial }
			clitest.CallMCP(t, session, "obit_graveyard", map[string]any{}, false, &graves)
			id := buried.Results[0].Burial.ID
			if len(graves.Burials) != 1 || graves.Burials[0].ID != id {
				t.Fatal("graveyard receipt", graves)
			}
			clitest.CallMCP(t, session, "obit_resurrect", map[string]any{"refs": []string{fmt.Sprint(id)}}, false, nil)
			if data, err := os.ReadFile(path); err != nil || string(data) != "local WorkBuddy MCP round trip" {
				t.Fatal("restored bytes", string(data), err)
			}
			clitest.CallMCP(t, session, "obit_graveyard", map[string]any{}, false, &graves)
			if len(graves.Burials) != 0 {
				t.Fatal("restored file still listed", graves)
			}
			state, err := store.Read()
			if err != nil || len(state.Proposals) != 0 || len(state.Items) != 0 {
				t.Fatal("explicit MCP flow produced automatic tracking", state, err)
			}
			if err := session.Close(); err != nil {
				t.Fatal(err)
			}
			plan, err = integrate.PlanWorkBuddyMCP(scope, "", true)
			if err != nil {
				t.Fatal(err)
			}
			if err := integrate.Apply(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}
