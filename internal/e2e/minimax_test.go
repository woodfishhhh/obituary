//go:build windows

package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func TestMiniMaxInstalledCommandWindows(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, store := commandRepo(t)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	root := t.TempDir()
	t.Setenv("MINIMAX_DATA_DIR", root)
	dir := filepath.Join(t.TempDir(), "中文 space")
	self := filepath.Join(dir, "obit.exe")
	binary, err := os.ReadFile(binaryPath(t, "obit"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(self, binary, 0o700); err != nil {
		t.Fatal(err)
	}
	changes, err := integrate.PlanMiniMax(integrate.Scope{Global: true}, integrate.Commands{Executable: self}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(changes); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "plugins", "obituary", "hooks", "hooks.json")
	writeFixture(t, filepath.Join(repo, "debug.log"), "owned scratch")
	// This harness reproduces runner.ts's shell:true commandWindows launch.
	// It runs the installed obit, without launching the mcode client.
	script := filepath.Join(t.TempDir(), "host.mjs")
	writeFixture(t, script, `import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
const [config,cwd]=process.argv.slice(2);
const hooks=JSON.parse(readFileSync(config,'utf8')).hooks;
for(const event of ['SessionStart','PostToolUse','Stop','SessionEnd']) {
 const handler=hooks[event][0].hooks[0];
 const payload={session_id:'installed-minimax',cwd,hook_event_name:event,source:'startup',tool_name:'write',tool_input:{path:'debug.log',content:'owned scratch'},tool_response:{content:[{type:'text',text:'done'}],details:{created:true,bytes_written:13}}};
 const result=spawnSync(handler.commandWindows,[],{shell:true,windowsHide:true,cwd,input:JSON.stringify(payload),encoding:'utf8',timeout:10000});
 assert.equal(result.status,0,result.stderr);
 assert.equal(result.stderr,'');
 if(event==='PostToolUse') assert.match(JSON.parse(result.stdout).hookSpecificOutput.additionalContext,/debug.log/);
 if(event==='Stop'||event==='SessionEnd') assert.equal(result.stdout,'');
}
`)
	cmd := exec.Command(node, script, config, repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native commandWindows: %s %v", out, err)
	}
	state, err := store.Read()
	if err != nil || len(state.Proposals) != 1 || len(state.Proposals[0].Notified) != 1 {
		t.Fatalf("proposal: %+v %v", state, err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, "debug.log")); err != nil || string(got) != "owned scratch" {
		t.Fatal("observation changed file", err)
	}
}

func TestMiniMaxInstalledMCPRelay(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, _ := commandRepo(t)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	root := t.TempDir()
	t.Setenv("MINIMAX_DATA_DIR", root)
	changes, err := integrate.PlanMiniMaxMCP(integrate.Scope{Global: true}, binaryPath(t, "obit"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(changes); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "plugins", "obituary-mcp")
	var config struct {
		Servers map[string]struct {
			Command string
			Args    []string
		} `json:"mcpServers"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "obituary.mcp.json"))
	if err != nil || json.Unmarshal(raw, &config) != nil {
		t.Fatal("MCP config", err)
	}
	server := config.Servers["obituary"]
	if server.Command != "node" || len(server.Args) != 1 {
		t.Fatalf("transport %+v", server)
	}
	// The native loader resolves explicit relative args to the plugin root.
	cmd := exec.Command(node, filepath.Join(dir, strings.TrimPrefix(server.Args[0], "./")))
	client := mcp.NewClient(&mcp.Implementation{Name: "minimax-test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	list, err := session.ListTools(t.Context(), nil)
	if err != nil || len(list.Tools) != 10 {
		t.Fatalf("inventory %+v %v", list, err)
	}
	path := filepath.Join(repo, "relay.log")
	writeFixture(t, path, "relay restore")
	var marked struct{ Results []core.Result }
	clitest.CallMCP(t, session, "obit_mark", map[string]any{"paths": []string{path}, "lifespan": "1d"}, false, &marked)
	if len(marked.Results) != 1 || marked.Results[0].Item == nil {
		t.Fatalf("mark %+v", marked)
	}
	clitest.CallMCP(t, session, "obit_bury", map[string]any{"paths": []string{path}}, false, nil)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file was not buried", err)
	}
	clitest.CallMCP(t, session, "obit_resurrect", map[string]any{"refs": []string{fmt.Sprint(marked.Results[0].Item.ID)}}, false, nil)
	if got, err := os.ReadFile(path); err != nil || string(got) != "relay restore" {
		t.Fatalf("restored bytes %q %v", got, err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
