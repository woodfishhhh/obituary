//go:build windows

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/integrate"
)

// The installed TS modules use the JS subset. Node executes the exact installed
// bytes under .mjs, calls the default host entrypoint and invokes the real obit.
func TestInstalledPluginDenialAndObservation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"pi", "omp", "opencode", "openclaw"} {
		t.Run(agent, func(t *testing.T) {
			repo, _ := commandRepo(t)
			node, err := exec.LookPath("node")
			if err != nil {
				t.Fatal(err)
			}
			a, ok := integrate.LookupAgent(agent)
			if !ok {
				t.Fatal("missing installer", agent)
			}
			plan, err := a.Plan(integrate.Scope{Project: repo, GuardRM: true}, integrate.Commands{Executable: binaryPath(t, "obit")}, false)
			if err != nil {
				t.Fatal(err)
			}
			err = integrate.Apply(plan)
			if agent == "openclaw" {
				if err == nil || !strings.Contains(err.Error(), "manual activation") {
					t.Fatalf("manual activation %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			folder := "extensions"
			if agent == "opencode" {
				folder = "plugins"
			}
			path := filepath.Join(repo, "."+agent, folder, "obituary.ts")
			if agent == "openclaw" {
				path = filepath.Join(repo, ".openclaw", "extensions", "obituary", "index.ts")
			}
			payload, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			module := strings.TrimSuffix(path, ".ts") + ".mjs"
			writeFixture(t, module, string(payload))
			script := filepath.Join(t.TempDir(), "host.mjs")
			writeFixture(t, script, pluginHostScript)
			writeFixture(t, filepath.Join(repo, "scratch.log"), "keep me")
			cmd := exec.Command(node, script, module, agent, repo)
			cmd.Env = append(os.Environ(), "OBIT_"+strings.ToUpper(agent)+"_LOCAL=1", "OBIT_"+strings.ToUpper(agent)+"_SHELL=bash", "OBIT_OPENCLAW_CWD="+repo)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("installed host protocol: %v\n%s", err, out)
			}
			if got, err := os.ReadFile(filepath.Join(repo, "scratch.log")); err != nil || string(got) != "keep me" {
				t.Fatalf("original command executed: %s %v", got, err)
			}
		})
	}
}

const pluginHostScript = `import assert from "node:assert/strict";
import { pathToFileURL } from "node:url";
const [path, agent, cwd] = process.argv.slice(2);
const module = await import(pathToFileURL(path).href);
assert.deepEqual(Object.keys(module), ["default"]);
const callbacks = {};
let denied;
if (agent === "opencode") {
  const hooks = await module.default.server({directory:cwd});
  try { await hooks["tool.execute.before"]({tool:"bash",sessionID:"installed"},{args:{command:"rm scratch.log"}}); }
  catch (error) { denied=error.message; }
  const output={output:"ok",metadata:{}};
  await hooks["tool.execute.after"]({tool:"write",sessionID:"installed",args:{filePath:"scratch.log"}},output);
  assert.match(output.output,/obit/);
  await hooks.event({event:{type:"session.idle",properties:{sessionID:"installed"}}});
} else {
  const api={on(name,fn){callbacks[name]=fn;},sendMessage(){}};
  if(agent==="omp")api.pi={settings:{}};
  module.default(api);
  if(agent==="openclaw") {
    const result=await callbacks.before_tool_call({toolName:"exec",params:{command:"rm scratch.log",workdir:cwd}},{sessionId:"installed"});
    assert.equal(result?.block,true); denied=result.blockReason;
    assert.equal(await callbacks.agent_end({}, {sessionId:"installed",workspaceDir:cwd}),undefined);
  } else {
    const ctx={cwd,sessionManager:{getSessionId(){return "installed";}}};
    const result=await callbacks.tool_call({toolName:"bash",input:{command:"rm scratch.log"}},ctx);
    assert.equal(result?.block,true);denied=result.reason;
    const observed=await callbacks.tool_result({toolName:"write",input:{path:"scratch.log"},isError:false,content:[]},ctx);
    assert.match(agent==="omp" ? observed.additionalContext : observed.content.at(-1).text,/obit/);
    assert.equal(await callbacks.agent_end({},ctx),undefined);
  }
}
assert.match(denied,/obit.*rm scratch\.log/);
console.log(agent+" installed bridge verified");
`
