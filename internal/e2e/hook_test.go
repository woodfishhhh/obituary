//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
	"strconv"
)

func TestHookProcessesNeverInvokeDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("real hook processes and fake Docker executable")
	}
	_, state, _ := testenv.FakeDocker(t)
	testenv.WriteDocker(t, state)
	parents := filepath.Join(t.TempDir(), "parents.log")
	t.Setenv("OBIT_FAKE_DOCKER_PARENTS", parents)
	root, _ := commandRepo(t)
	var hookPIDs []string
	for _, event := range []string{"PreToolUse", "SessionStart", "PostToolUse", "Stop"} {
		payload, _ := json.Marshal(map[string]any{"hook_event_name": event, "session_id": "docker-count", "cwd": root, "tool_name": "Bash", "tool_input": map[string]string{"command": "docker run alpine"}})
		cmd := exec.Command(binaryPath(t, "obit"), "hook", "claude")
		cmd.Stdin = bytes.NewReader(payload)
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		hookPIDs = append(hookPIDs, strconv.Itoa(cmd.Process.Pid))
		if err := cmd.Wait(); err != nil {
			t.Fatal(output.String(), err)
		}
	}
	// Drain the background finish through its shared ledger before inspecting counts.
	_, _, err := execute(t, "hook", "finish", "--project", root, "--session", "docker-count")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(parents)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, pid := range strings.Fields(string(data)) {
		for _, hookPID := range hookPIDs {
			if pid == hookPID {
				t.Fatalf("hook process invoked Docker: %s", pid)
			}
		}
	}
}

func TestCodexDeniedPathsFailOpenWithDiagnostics(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	self := binaryPath(t, "obit")
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, event, target, operation string
		access                         uint32
	}{
		{"cwd-start", "SessionStart", "cwd", "resolve project directory", windows.GENERIC_READ},
		{"cwd-stop", "Stop", "cwd", "resolve project directory", windows.GENERIC_READ},
		{"cwd-end", "SessionEnd", "cwd", "resolve project directory", windows.GENERIC_READ},
		{"snapshot", "SessionStart", "sessions", "snapshot", windows.FILE_WRITE_DATA},
		{"ledger-lock", "SessionStart", "store.lock", "lock store.lock", windows.FILE_WRITE_DATA},
		{"ledger-read", "SessionStart", "store.json", "read store", windows.FILE_READ_DATA},
		{"write-deferred", "PostToolUse", "store.lock", "store.lock", windows.FILE_WRITE_DATA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, store := clitest.CommandRepo(t)
			path := filepath.Join(root, "debug.log")
			writeFixture(t, path, "user content")
			if err := store.Update(func(*core.State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			sessions := filepath.Join(store.Home, "sessions")
			if err := os.Mkdir(sessions, 0o700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(store.Home, tc.target)
			if tc.target == "cwd" {
				target = root
			}
			t.Run("denied", func(t *testing.T) {
				clitest.DenyAccess(t, target, tc.access)
				if tc.target == "cwd" {
					// Attribute queries can succeed while CreateFile in Resolve fails.
					if _, err := os.Stat(root); err != nil {
						t.Fatalf("fixture must permit Stat: %v", err)
					}
					if _, _, err := project.Find(root); !errors.Is(err, os.ErrPermission) {
						t.Fatalf("fixture must deny project resolution: %v", err)
					}
				}
				payload, _ := json.Marshal(map[string]any{
					"hook_event_name": tc.event, "session_id": "denied-session", "cwd": root,
					"tool_name": "Write", "tool_input": map[string]string{"file_path": path},
				})
				command := exec.Command(self, "hook", "codex")
				// Do not make the denied payload cwd the process's own cwd.
				command.Dir = filepath.Dir(root)
				command.Stdin = bytes.NewReader(payload)
				out, err := command.CombinedOutput()
				if err != nil || len(out) != 0 {
					t.Fatalf("hook must exit zero silently: %q %v", out, err)
				}
				data, err := os.ReadFile(filepath.Join(store.Home, "hook.log"))
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("hook.log: %s", data)
				// %q escapes Windows separators and keeps each error on one line.
				log := strings.ReplaceAll(string(data), `\\`, `\`)
				for _, want := range []string{"codex " + tc.event, target, tc.operation, "session=", "denied-session", "cwd=", "home=", account.Uid, "hook continues", "obit doctor"} {
					if !strings.Contains(log, want) {
						t.Errorf("diagnostic missing %q: %s", want, data)
					}
				}
				assertPermissionMetadata(t, string(data), target)
			})
			if data, err := os.ReadFile(path); err != nil || string(data) != "user content" {
				t.Fatalf("denied hook changed user content: %q %v", data, err)
			}
			state, err := store.Read()
			if err != nil || len(state.Items) != 0 || len(state.Proposals) != 0 {
				t.Fatalf("denied work must not register: %+v %v", state, err)
			}
			snaps, err := filepath.Glob(filepath.Join(sessions, "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			wantSnapshot := tc.name == "ledger-lock" || tc.name == "ledger-read"
			if (len(snaps) == 1) != wantSnapshot || len(snaps) > 1 {
				t.Errorf("snapshot count=%d, want present=%v", len(snaps), wantSnapshot)
			}
			if store.HasDeferredObservations() != (tc.event == "PostToolUse") {
				t.Error("only the failed write must be deferred")
			}
			if tc.event == "PostToolUse" {
				svc, err := app.Open()
				if err != nil {
					t.Fatal(err)
				}
				// A consumer that finds the replay lock busy leaves the queue to
				// its holder (here the exiting isolated runner), so poll.
				for deadline := time.Now().Add(10 * time.Second); store.HasDeferredObservations() && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
					if err := app.Engine(svc).ReplayObservations(); err != nil {
						t.Fatal(err)
					}
				}
				state, err := store.Read()
				if err != nil || len(state.Proposals) != 1 || store.HasDeferredObservations() {
					t.Fatalf("restored access did not replay the write: %+v %v", state, err)
				}
			}
			if out, err := exec.Command(self, "daemon", "stop").CombinedOutput(); err != nil {
				t.Fatalf("stop isolated daemon: %s %v", out, err)
			}
		})
	}
}

func assertPermissionMetadata(t *testing.T, log, target string) {
	t.Helper()
	for _, line := range strings.Split(log, "\n") {
		if !strings.Contains(line, " codex ") {
			continue // The runner can also log a background tick failure.
		}
		summary, denied := hook.PermissionSummary(line)
		if !denied {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal([]byte(summary), &fields); err != nil {
			t.Fatalf("hook did not emit structured permission metadata: %s", summary)
		}
		for _, name := range []string{"user", "sid", "integrity_sid", "app_container", "restricted", "elevated", "parent_pid", "parent_image", "operation", "path"} {
			if value, ok := fields[name]; !ok || value == nil || value == "" {
				t.Errorf("permission metadata missing %s: %s", name, summary)
			}
		}
		path, _ := fields["path"].(string)
		if !filepath.IsAbs(path) || !strings.HasPrefix(path, target) {
			t.Errorf("permission failure path does not identify target %q: %s", target, summary)
		}
		return
	}
	t.Fatal("permission diagnostic missing")
}

func TestCodexUnwritableHomeStillAllowsAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	self := binaryPath(t, "obit")
	root, store := clitest.CommandRepo(t)
	if err := os.Mkdir(store.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Run("denied", func(t *testing.T) {
		clitest.DenyAccess(t, store.Home, windows.FILE_WRITE_DATA|windows.FILE_APPEND_DATA)
		for _, event := range []string{"SessionStart", "PostToolUse"} {
			path := filepath.Join(root, "debug.log")
			writeFixture(t, path, "user content")
			payload, _ := json.Marshal(map[string]any{
				"hook_event_name": event, "session_id": "unwritable", "cwd": root,
				"tool_name": "Write", "tool_input": map[string]string{"file_path": path},
			})
			command := exec.Command(self, "hook", "codex")
			command.Stdin = bytes.NewReader(payload)
			if out, err := command.CombinedOutput(); err != nil || len(out) != 0 {
				t.Fatalf("unwritable data/log must still exit zero silently: %q %v", out, err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "user content" {
				t.Fatalf("unwritable data changed user file: %q %v", data, err)
			}
		}
	})
	entries, err := os.ReadDir(store.Home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("denied home must have no snapshot, queue, ledger or log: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".obit")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hook created an alternate project ledger: %v", err)
	}
	if out, err := exec.Command(self, "daemon", "stop").CombinedOutput(); err != nil {
		t.Fatalf("stop isolated daemon: %s %v", out, err)
	}
}

func TestPreToolUseCLIWithoutConfigOrLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			_, store := commandRepo(t)
			writeFixture(t, filepath.Join(store.Home, "store.json"), "broken ledger")
			writeFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "broken config [")
			self := binaryPath(t, "obit")
			for _, payload := range []string{`{"hook_event_name":"PreToolUse","cwd":"C:/repo","tool_name":"Bash","tool_input":{"command":"git clean -fd build","shell":"bash"}}`, "malformed"} {
				command := exec.Command(self, "hook", agent)
				command.Env = append(os.Environ(), "PATH=", "OBIT_GUARD_RM=1", "OBIT_HOOK_AUDIT=0")
				command.Stdin = strings.NewReader(payload)
				out, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("hook exit must be zero: %s %v", out, err)
				}
				if payload == "malformed" {
					if len(out) != 0 {
						t.Fatalf("malformed input must be silent: %s", out)
					}
				} else if !bytes.Contains(out, []byte(`"permissionDecision":"deny"`)) {
					t.Fatalf("config/ledger affected PreToolUse: %s", out)
				}
			}
		})
	}
}

func TestRegisteredAgentCLIProtocols(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	self := binaryPath(t, "obit")
	fixtures := map[string]string{
		"antigravity": "antigravity-pre", "cline": "cline-pre", "copilot": "copilot-cli-pre",
		"cursor": "cursor-pre", "droid": "droid-pre", "gemini": "gemini-pre",
		"hermes": "hermes-pre", "kilocode": "kilocode-pre", "kimi": "kimi-pre",
		"omp": "omp-protocol", "openclaw": "openclaw-protocol", "opencode": "opencode-protocol",
		"pi": "pi-protocol", "trae": "trae-pre",
		"codebuddy": "codebuddy-cli-pre", "codebuddy-ide": "codebuddy-ide-pre", "trae-cn": "trae-pre", "vibe": "vibe-pre",
		"qoder": "qoder-cli-pre", "qoder-cn": "qoder-cn-pre", "lingma": "lingma-pre",
		"deepseek-harness": "deepseek-pwsh-pre",
		"minimax-code":     "minimax-native-pre",
		"zcode":            "zcode-pre",
	}
	if len(hook.Agents()) != len(fixtures)+2 {
		t.Fatalf("CLI fixture inventory differs from registry: %d fixtures, %d agents", len(fixtures), len(hook.Agents()))
	}
	for _, agent := range hook.Agents() {
		if agent.Name == "claude" || agent.Name == "codex" || agent.Name == "test-native" {
			continue
		}
		t.Run(agent.Name, func(t *testing.T) {
			fixture, ok := fixtures[agent.Name]
			if !ok {
				t.Fatalf("registered agent has no CLI fixture: %s", agent.Name)
			}
			raw, err := os.ReadFile(filepath.Join("..", "hook", "testdata", fixture+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var input map[string]any
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			if cases, ok := input["cases"].([]any); ok {
				input = cases[0].(map[string]any)["bridge"].(map[string]any)
			}
			root := t.TempDir()
			home, config := filepath.Join(root, "data"), filepath.Join(root, "config")
			t.Setenv("OBIT_HOME", home)
			t.Setenv("OBIT_CONFIG_DIR", config)
			t.Setenv("OBIT_HOOK_AUDIT", "1")
			t.Setenv("PATH", t.TempDir())
			writeFixture(t, filepath.Join(root, ".git"), "invalid git pointer")
			writeFixture(t, filepath.Join(config, "config.toml"), "[broken")
			writeFixture(t, filepath.Join(home, "store.json"), "broken ledger")
			input["cwd"] = root
			input["workspace_roots"] = []string{root}
			input["workspacePaths"] = []string{root}
			raw, _ = json.Marshal(input)
			invocation := hook.Invocation{Agent: agent.Name}
			args := []string{agent.Name}
			if agent.Name == "copilot" || agent.Name == "antigravity" {
				invocation.Event = "PreToolUse"
				if agent.Name == "copilot" {
					invocation.Event = "preToolUse"
				}
				args = append(args, "--event", invocation.Event)
			}
			ev, err := hook.Decode(agent.Adapter, raw, invocation)
			if err != nil || ev.Name != "PreToolUse" {
				t.Fatalf("fixture does not exercise pre path: %+v %v", ev, err)
			}
			if ev.Shell == guardrm.Uncertain && ev.SkipReason == "" {
				// These fixtures omit executor evidence, so a denial requires
				// the owner's declaration. Other agents keep the broken config.
				shell := map[string]string{"codebuddy": "bash", "codebuddy-ide": "bash", "minimax-code": "powershell"}[agent.Name]
				if shell == "" {
					t.Fatalf("fixture needs an explicit shell declaration: %s", agent.Name)
				}
				writeFixture(t, filepath.Join(config, "config.toml"), "[agent_shells]\n"+agent.Name+" = '"+shell+"'\n")
			}
			command := exec.Command(self, append([]string{"hook"}, args...)...)
			command.Stdin = bytes.NewReader(append([]byte("\xef\xbb\xbf"), raw...))
			data, err := command.CombinedOutput()
			out := string(data)
			if err != nil {
				t.Fatal(err)
			}
			if ev.SkipReason == "" {
				if !strings.Contains(out, "deny") || !strings.Contains(out, "restorable deletion") {
					t.Fatalf("CLI lost intentional denial: %s", out)
				}
			} else if out != string(agent.Adapter.Fallback(ev)) {
				t.Fatalf("CLI lost neutral response: %s", out)
			}
			for _, name := range []string{"store.lock", "hook.log", "sessions"} {
				if _, err := os.Stat(filepath.Join(home, name)); !os.IsNotExist(err) {
					t.Fatalf("pre event initialized %s: %v", name, err)
				}
			}
			if data, err := os.ReadFile(filepath.Join(home, "store.json")); err != nil || string(data) != "broken ledger" {
				t.Fatalf("pre event touched ledger: %s %v", data, err)
			}
			if data, err := os.ReadFile(filepath.Join(home, "delete-guard.jsonl")); err != nil || len(data) == 0 {
				t.Fatalf("CLI did not reach guard: %s %v", data, err)
			}
		})
	}
}

func TestDeclaredAgentShellCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	self := binaryPath(t, "obit")
	for _, agent := range []string{"codebuddy", "codebuddy-ide", "minimax-code"} {
		t.Run(agent, func(t *testing.T) {
			for _, tc := range []struct{ name, shell, command, equivalent, cwd, backend string }{
				{name: "unset", command: "rm -rf 'scratch file'"},
				{name: "bash", shell: "bash", command: "rm -rf 'scratch file'", equivalent: "rm -rf 'scratch file'"},
				{name: "powershell", shell: "powershell", command: "Remove-Item -LiteralPath 'scratch file' -Force", equivalent: "remove-item -LiteralPath 'scratch file' -Force"},
				{name: "invalid-config", shell: "cmd", command: "rm -rf 'scratch file'"},
				{name: "remote-cwd", shell: "bash", command: "rm -rf 'scratch file'", cwd: "/mnt/c/repo"},
				{name: "remote-backend", shell: "bash", command: "rm -rf 'scratch file'", backend: "ssh"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					root := t.TempDir()
					home, config := filepath.Join(root, "data"), filepath.Join(root, "config")
					t.Setenv("OBIT_HOME", home)
					t.Setenv("OBIT_CONFIG_DIR", config)
					t.Setenv("OBIT_GUARD_RM", "1")
					t.Setenv("OBIT_HOOK_AUDIT", "0")
					t.Setenv("PATH", t.TempDir())
					if tc.shell != "" {
						writeFixture(t, filepath.Join(config, "config.toml"), "[agent_shells]\n"+agent+" = '"+tc.shell+"'\n")
					}
					writeFixture(t, filepath.Join(root, ".git"), "invalid git pointer")
					writeFixture(t, filepath.Join(root, "scratch file"), "unchanged")
					writeFixture(t, filepath.Join(home, "store.json"), "broken ledger")
					cwd := root
					if tc.cwd != "" {
						cwd = tc.cwd
					}
					payload, err := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": cwd, "backend": tc.backend,
						"tool_name": "Bash", "tool_input": map[string]any{"command": tc.command}})
					if err != nil {
						t.Fatal(err)
					}
					command := exec.Command(self, "hook", agent)
					command.Stdin = bytes.NewReader(payload)
					out, err := command.CombinedOutput()
					if err != nil {
						t.Fatalf("hook exit must be zero: %s %v", out, err)
					}
					if tc.equivalent == "" {
						if len(out) != 0 {
							t.Fatalf("command must pass through: %s", out)
						}
					} else {
						var response struct {
							HookSpecificOutput struct{ PermissionDecision, PermissionDecisionReason string }
						}
						if err := json.Unmarshal(out, &response); err != nil {
							t.Fatalf("invalid hook response: %s %v", out, err)
						}
						decision := response.HookSpecificOutput
						if decision.PermissionDecision != "deny" || !strings.Contains(decision.PermissionDecisionReason, " "+tc.equivalent+"`") {
							t.Fatalf("declared shell lost equivalent denial: %s", out)
						}
					}
					for path, want := range map[string]string{filepath.Join(root, "scratch file"): "unchanged", filepath.Join(home, "store.json"): "broken ledger"} {
						if got, err := os.ReadFile(path); err != nil || string(got) != want {
							t.Fatalf("hook changed %s: %s %v", path, got, err)
						}
					}
					if _, err := os.Stat(filepath.Join(home, "store.lock")); !os.IsNotExist(err) {
						t.Fatalf("hook opened ledger: %v", err)
					}
				})
			}
		})
	}
}
