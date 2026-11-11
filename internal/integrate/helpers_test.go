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
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func ownershipCommands(agent string) Commands {
	return Commands{Native: `"C:/tools/obit.exe" hook ` + agent, Executable: `C:\tools\obit.exe`}
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
	changes, err := PlanClaude(scope, `"C:/tools/obit.exe" hook claude`, uninstall)
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

const (
	unixCmd = "/mnt/d/tools/obit.exe hook codex"
	winCmd  = `"D:/tools/obit.exe" hook codex`
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
	e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}, Guard: core.Guard{System: []string{home}}, FS: winfs.FS{}}, Trust: rules.TrustStore{Path: filepath.Join(home, "trust.json")}}
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
		payload, err = pluginPayload(Scope{Project: dir}, module, hermesPlugin, `C:\test\obit.exe`)
	case "kilocode":
		payload, err = pluginPayload(Scope{Project: dir}, module, kiloCodePlugin, `C:\test\obit.exe`)
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
