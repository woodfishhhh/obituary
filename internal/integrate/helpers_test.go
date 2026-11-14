package integrate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func testBackupPath(t *testing.T, path string) string {
	t.Helper()
	result, err := userdirs.ReceiptPath(path, "integration-backups", ".bak")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func testAssetReceiptPath(t *testing.T, path string) string {
	t.Helper()
	result, err := userdirs.ReceiptPath(path, "integration-receipts", ".obituary-asset")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func checkAgentSkillTargets(t *testing.T, agent string) {
	t.Helper()
	names := []string{agent}
	if agent == "qoder" || agent == "trae" {
		names = append(names, agent+"-cn")
	}
	for _, name := range names {
		for _, global := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/global=%t", name, global), func(t *testing.T) {
				root := agentGoldenEnvironment(t)
				s := Scope{Global: global, Project: filepath.Join(root, "project")}
				if name == "qoderwork" && !global || name == "windsurf" && global {
					a, _ := LookupAgent(name)
					if _, err := a.PlanSkill(s, false); err == nil {
						t.Fatal("accepted undocumented project skill")
					}
					return
				}
				checkSkillLifecycle(t, name, s, expectedSkillRoot(root, name, global))
			})
		}
	}
}

func expectedSkillRoot(root, name string, global bool) string {
	if !global {
		folder := ".agents"
		switch name {
		case "lingma":
			folder = ".lingma"
		case "qoder", "qoder-cn":
			folder = ".qoder"
		case "trae":
			folder = ".trae"
		case "windsurf":
			folder = ".windsurf"
		}
		return filepath.Join(root, "project", folder, "skills")
	}
	switch name {
	case "antigravity":
		return filepath.Join(root, "OBIT_USER_HOME", ".gemini", "config", "skills")
	case "hermes":
		return filepath.Join(root, "HERMES_HOME", "skills")
	case "openclaw":
		return filepath.Join(root, "OPENCLAW_STATE_DIR", "skills")
	case "qoder":
		return filepath.Join(root, "QODER_CONFIG_DIR", "skills")
	case "qoder-cn":
		return filepath.Join(root, "QODERCN_CONFIG_DIR", "skills")
	case "lingma", "trae", "trae-cn", "qoderwork":
		return filepath.Join(root, "OBIT_USER_HOME", "."+name, "skills")
	default:
		return filepath.Join(root, "OBIT_USER_HOME", ".agents", "skills")
	}
}

func applyPreservedSkill(t *testing.T, plan []Change, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if PreservedError(plan) == nil {
		t.Fatal("missing user-content preservation report")
	}
	if err := Apply(plan); err == nil {
		t.Fatal("unresolved preservation must be reported")
	}
}

func checkSkillLifecycle(t *testing.T, agent string, s Scope, root string) {
	t.Helper()
	a, ok := LookupAgent(agent)
	if !ok || a.PlanSkill == nil {
		t.Fatal("missing skill planner", agent)
	}
	path := filepath.Join(root, "obituary", "SKILL.md")
	plan, err := a.PlanSkill(s, false)
	if err != nil || len(plan) == 0 || plan[0].Path != path {
		t.Fatalf("skill path: %v %v; want %s", plan, err, path)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("plan wrote skill", err)
	}
	applyIntegration(t, plan, nil)
	if read(t, path) != string(skillContent) {
		t.Fatal("skill bytes changed")
	}
	plan, err = a.PlanSkill(s, false)
	if err != nil || Pending(plan) {
		t.Fatal("skill install is not idempotent", err)
	}
	status, err := InspectExtension(s, agent, "skill", "obit")
	if err != nil || !status.Installed || !status.Current {
		t.Fatalf("skill status: %+v %v", status, err)
	}
	plan, err = a.PlanSkill(s, true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("owned skill retained", err)
	}
	checkSkillUserContent(t, a, s, path)
}

func checkSkillUserContent(t *testing.T, a Agent, s Scope, path string) {
	t.Helper()
	for _, owned := range []bool{false, true} {
		if owned {
			// The previous unowned file belongs to this test fixture.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			plan, err := a.PlanSkill(s, false)
			applyIntegration(t, plan, err)
		}
		writeIntegrationFile(t, path, "user skill")
		if _, err := a.PlanSkill(s, false); err == nil {
			t.Fatal("accepted user content", a.Name)
		}
		plan, err := a.PlanSkill(s, true)
		if err != nil {
			t.Fatal(err)
		}
		_ = Apply(plan)
		if read(t, path) != "user skill" {
			t.Fatal("removed user skill", a.Name)
		}
	}
}

func checkSharedSkillOwners(t *testing.T, s Scope, names ...string) {
	t.Helper()
	var combined []Change
	for _, name := range names {
		a, _ := LookupAgent(name)
		plan, err := a.PlanSkill(s, false)
		if err != nil {
			t.Fatal(err)
		}
		combined = append(combined, plan...)
	}
	plan, err := ComposeAgentChanges(combined)
	applyIntegration(t, plan, err)
	path := plan[0].Path
	for i, name := range names {
		status, err := InspectExtension(s, name, "skill", "obit")
		if err != nil || !status.Installed || !status.Current {
			t.Fatalf("shared skill status for %s: %+v %v", name, status, err)
		}
		a, _ := LookupAgent(name)
		plan, err := a.PlanSkill(s, true)
		applyIntegration(t, plan, err)
		if i == len(names)-1 {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("last owner retained skill", err)
			}
		} else if read(t, path) != string(skillContent) {
			t.Fatal("removed another owner's skill")
		}
	}
}

func ownershipCommands(agent string) Commands {
	return Commands{Native: testenv.NativeFixture(`"C:/tools/obit.exe" hook `) + agent, Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}
}

const userSettings = "\xef\xbb\xbf" + `{
  "theme": "dark",
  "hooks": {
    "PostToolUse": [
      { "matcher": "Bash", "hooks": [ { "type": "command", "command": "my-linter" } ] }
    ]
  },
  "model": "opus"
}`

func setup(t *testing.T) (dir string, scope Scope) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(userSettings), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# My rules\n\nBe brief.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, Scope{Global: true}
}

func install(t *testing.T, scope Scope, uninstall bool) []Change {
	t.Helper()
	changes, err := PlanClaude(scope, testenv.NativeFixture(`"C:/tools/obit.exe" hook claude`), uninstall)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	return changes
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

var (
	unixCmd = "/mnt/d/tools/obit.exe hook codex"
	winCmd  = testenv.NativeFixture(`"D:/tools/obit.exe" hook codex`)
)

// The extra first write models an editor save after Apply's shared preflight,
// using the same deterministic interleaving as TestApplyRechecksBeforeReplace.
func editAfterPreflight(t *testing.T, changes []Change, path, content string) []Change {
	t.Helper()
	for _, c := range changes {
		if c.Path == path {
			return append([]Change{{Path: path, Action: "update", existed: c.existed, orig: c.orig, data: []byte(content)}}, changes...)
		}
	}
	t.Fatalf("no planned change for %s", path)
	return nil
}

func makeIntegrationJunction(t *testing.T, path, target string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		return
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", path, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
}

func writeIntegrationFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func applyIntegration(t *testing.T, plan []Change, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
}

func pluginEngine(t *testing.T, approved bool) (string, *auto.Engine) {
	t.Helper()
	dir := t.TempDir()
	root, home := filepath.Join(dir, "repo"), filepath.Join(dir, "home")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	testenv.InitRepo(t, root)
	// Plugins report writes; automatic registration must be on to see them.
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}, Guard: core.Guard{System: []string{home}}, FS: winfs.FS{}, ProposalMode: core.ProposalsTiered}, Trust: rules.TrustStore{Path: filepath.Join(home, "trust.json")}}
	e.Svc.Owner, e.Svc.Recheck = auto.Owner, e.RecheckRules
	if approved {
		if err := e.Svc.Store.Update(func(st *core.State) error {
			st.Projects = map[string]core.ProjectState{core.ProjectKey(root): {Consent: core.ConsentApproved}}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return root, e
}

// Execute the installed plugin. Only its process transport is replaced: each
// request crosses stdout into the real Go hook and its reply returns on stdin.
func pluginRoundTrip(t *testing.T, agent, root string, e *auto.Engine, fields map[string]any) []byte {
	t.Helper()
	dir := t.TempDir()
	var payload []byte
	var err error
	module, interpreter := filepath.Join(dir, "plugin.mjs"), "node"
	args := []string{"--experimental-vm-modules", "plugins/bridge/roundtrip.mjs"}
	switch agent {
	case "hermes":
		module, interpreter = filepath.Join(dir, "plugin.py"), "python"
		args = []string{"-B", "plugins/hermes/roundtrip.py"}
		payload, err = pluginPayload(Scope{Project: dir}, module, hermesPlugin, testenv.NativeFixture(`C:\test\obit.exe`))
	case "kilocode":
		payload, err = pluginPayload(Scope{Project: dir}, module, kiloCodePlugin, testenv.NativeFixture(`C:\test\obit.exe`))
	default:
		host := agent
		if host == "omp" {
			host = "pi"
		}
		payload, err = pluginFiles.ReadFile("plugins/" + host + "/index.ts")
		if err == nil {
			bridge, readErr := pluginFiles.ReadFile("plugins/bridge/transport.ts")
			err = readErr
			payload = bytes.ReplaceAll(payload, []byte("/* OBIT_BRIDGE */"), bridge)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, module, string(payload))
	fields["agent"], fields["cwd"] = agent, root
	config := filepath.Join(dir, "input.json")
	writeIntegrationFile(t, config, string(mustJSON(fields)))
	if interpreter == "python" {
		interpreter = testenv.PythonCommand()
	}
	cmd := exec.CommandContext(t.Context(), interpreter, append(args, module, config)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	var result []byte
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var packet struct {
			Request json.RawMessage `json:"request"`
			Result  json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &packet); err != nil {
			t.Fatal(err)
		}
		if packet.Request != nil {
			var out bytes.Buffer
			hook.Run(agent, e, bytes.NewReader(packet.Request), &out, &hook.Runner{})
			if out.Len() == 0 {
				out.WriteString("{}\n")
			}
			if _, err := stdin.Write(out.Bytes()); err != nil {
				t.Fatal(err)
			}
		} else {
			result = packet.Result
		}
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil || scanner.Err() != nil || result == nil {
		t.Fatalf("plugin round trip: %v %v %s", err, scanner.Err(), stderr.String())
	}
	return result
}

func pluginSweepKeepsNeighbor(t *testing.T, e *auto.Engine, neighbor string) {
	t.Helper()
	e.Svc.Now = func() time.Time { return time.Now().Add(30 * 24 * time.Hour) }
	results, err := e.Svc.Sweep(false)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(neighbor); err != nil || string(data) != "untouched" {
		t.Fatalf("untouched neighbor was moved: %v; sweep=%s", err, fmt.Sprint(results))
	}
}

func testPluginContext(t *testing.T, agent string) {
	t.Helper()
	for _, granted := range []bool{false, true} {
		if !granted && agent != "openclaw" {
			continue
		}
		t.Run(fmt.Sprint(granted), func(t *testing.T) {
			root, e := pluginEngine(t, false)
			writeIntegrationFile(t, filepath.Join(root, "debug.log"), "observed write")
			result := pluginRoundTrip(t, agent, root, e, map[string]any{"granted": granted})
			st, err := e.Svc.Store.Read()
			if err != nil || len(st.Proposals) != 1 {
				t.Fatalf("plugin observation missing: %+v %v", st, err)
			}
			if strings.Contains(string(result), "debug.log") != granted || (len(st.Proposals[0].Notified) == 1) != granted {
				t.Fatalf("plugin context not delivered through host callback: %s %+v", result, st.Proposals)
			}
		})
	}
}
