//go:build windows

package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func TestAgentsWriteObservation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range hook.Agents() {
		t.Run(agent.Name, func(t *testing.T) {
			_, repo := lifecycleEnvironment(t)
			lifecycleLocalBackend(t)
			input, event := lifecyclePayload(t, agent.Name, false, repo)
			path := filepath.Join(repo, "debug.log")
			writeFixture(t, path, "user bytes\x00\xff\r\n中文")
			out := lifecycleHook(t, agent.Name, input, event)
			lifecycleDrainQueue(t)
			state := lifecycleState(t)
			if agent.Capabilities.WriteObservation.Level == hook.Unsupported {
				if len(state.Items)+len(state.Proposals) != 0 {
					t.Fatalf("unsupported write registered local data: %+v", state)
				}
				t.Logf("write observation unavailable: %s", agent.Capabilities.WriteObservation.Detail)
			} else {
				if len(state.Items) != 0 || len(state.Proposals) != 1 || len(state.Proposals[0].Candidates) != 1 || state.Proposals[0].Candidates[0].Path != path {
					t.Fatalf("write did not propose exact fixture file: output=%s state=%+v", out, state)
				}
				if state.Proposals[0].Project != repo || state.Proposals[0].Session == "" {
					t.Fatalf("proposal lost project/session: %+v", state.Proposals[0])
				}
				lifecycleCLI(t, nil, 0, "approve", strconv.FormatInt(state.Proposals[0].ID, 10))
				state = lifecycleState(t)
				if len(state.Items) != 1 || state.Items[0].Path != path || len(state.Proposals) != 0 {
					t.Fatalf("proposal did not become tracking: %+v", state)
				}
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "user bytes\x00\xff\r\n中文" {
				t.Fatalf("write observation modified content: %q %v", data, err)
			}
		})
	}
}

func TestAgentsDeleteGuardResurrection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range hook.Agents() {
		t.Run(agent.Name, func(t *testing.T) {
			_, repo := lifecycleEnvironment(t)
			lifecycleLocalBackend(t)
			input, event := lifecyclePayload(t, agent.Name, true, repo)
			build := filepath.Join(repo, "build")
			want := demoBuildTree(t, build)
			if slices.Contains([]string{"codebuddy", "codebuddy-ide", "minimax-code"}, agent.Name) {
				lifecycleNeutral(t, agent.Name, lifecycleHook(t, agent.Name, input, event))
				demoAssertTree(t, build, want)
				lifecycleNoTracking(t)
				writeFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[agent_shells]\n"+agent.Name+" = 'bash'\n")
			}
			if _, bridge := input["version"]; bridge {
				for key, missing := range map[string]any{"local": false, "shell": ""} {
					original := input[key]
					input[key] = missing
					lifecycleNeutral(t, agent.Name, lifecycleHook(t, agent.Name, input, event))
					input[key] = original
					demoAssertTree(t, build, want)
					lifecycleNoTracking(t)
				}
			}
			if agent.Name == "deepseek-harness" {
				for _, key := range []string{"OBIT_DEEPSEEK_LOCAL", "OBIT_DEEPSEEK_SHELL"} {
					t.Run("undeclared-"+key, func(t *testing.T) {
						t.Setenv(key, "")
						lifecycleNeutral(t, agent.Name, lifecycleHook(t, agent.Name, input, event))
						demoAssertTree(t, build, want)
						lifecycleNoTracking(t)
					})
				}
			}
			out := lifecycleHook(t, agent.Name, input, event)
			demoAssertTree(t, build, want)
			demoGraves(t)
			canGuard := agent.Capabilities.DeleteGuard.Level != hook.Unsupported && !slices.Contains([]string{"qoder", "qoder-cn", "lingma", "zcode"}, agent.Name)
			if !canGuard {
				lifecycleNeutral(t, agent.Name, out)
				t.Logf("deletion passed through: %s", agent.Capabilities.DeleteGuard.Detail)
				return
			}
			reason := lifecycleDenial(t, out)
			parts := strings.Split(reason, "`")
			self, err := filepath.EvalSymlinks(binaryPath(t, "obit"))
			if err != nil {
				t.Fatal(err)
			}
			command := "'" + strings.ReplaceAll(filepath.ToSlash(self), "'", `'\''`) + "' rm -rf build"
			if len(parts) < 3 || parts[1] != command {
				t.Fatalf("non-equivalent deletion suggestion: %q; want %q", reason, command)
			}
			bash := lifecycleBash(t)
			cmd := exec.Command(bash, "--noprofile", "--norc", "-c", parts[1])
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("exact suggested command failed: %s %v", data, err)
			}
			demoAbsent(t, build)
			grave := demoGraves(t, build)[build]
			if !strings.HasPrefix(grave.Grave, os.Getenv("OBIT_HOME")+string(filepath.Separator)) {
				t.Fatalf("burial escaped isolated home: %s", grave.Grave)
			}
			demoAssertTree(t, grave.Grave, want)
			lifecycleCLI(t, nil, 0, "resurrect", "--last")
			demoAssertTree(t, build, want)
			demoGraves(t)
			t.Logf("denied rm -rf; executed exact suggestion via %s; restored all %d tree entries", bash, len(want))
		})
	}
}

func TestCodeBuddySharedGuardDeclaration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	_, repo := lifecycleEnvironment(t)
	input, _ := lifecyclePayload(t, "codebuddy-ide", true, repo)
	plan, err := integrate.PlanCodeBuddyIDE(integrate.Scope{Project: repo, GuardRM: true}, integrate.Commands{Executable: binaryPath(t, "obit")}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(plan); err != nil {
		t.Fatal(err)
	}
	var installed struct {
		Hooks map[string][]struct{ Hooks []struct{ Command string } }
	}
	data, err := os.ReadFile(filepath.Join(repo, ".codebuddy", "settings.json"))
	if err != nil || json.Unmarshal(data, &installed) != nil || len(installed.Hooks["PreToolUse"]) != 1 || len(installed.Hooks["PreToolUse"][0].Hooks) != 1 {
		t.Fatalf("installed guard command missing: %s %v", data, err)
	}
	command := installed.Hooks["PreToolUse"][0].Hooks[0].Command
	run := func() string {
		t.Helper()
		raw, _ := json.Marshal(input)
		cmd := exec.Command(lifecycleBash(t), "--noprofile", "--norc", "-c", command)
		cmd.Stdin = strings.NewReader(string(raw))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("installed hook: %s %v", out, err)
		}
		return string(out)
	}
	build := filepath.Join(repo, "build")
	want := demoBuildTree(t, build)
	path := filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml")
	// Both hosts install the same hook codebuddy command; native IDE tool
	// names must still use the IDE's separate declaration.
	writeFixture(t, path, "[agent_shells]\ncodebuddy = 'bash'\n")
	lifecycleNeutral(t, "codebuddy", run())
	writeFixture(t, path, "[agent_shells]\ncodebuddy = 'powershell'\ncodebuddy-ide = 'bash'\n")
	lifecycleDenial(t, run())
	demoAssertTree(t, build, want)
	lifecycleNoTracking(t)
}

func TestAgentsRemoteObservationIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range hook.Agents() {
		t.Run(agent.Name, func(t *testing.T) {
			_, repo := lifecycleEnvironment(t)
			lifecycleLocalBackend(t)
			path := filepath.Join(repo, "debug.log")
			writeFixture(t, path, "local namesake must survive")
			for _, prefix := range []string{"vscode-remote://ssh-remote+obit-test/", "https://cloud.invalid/workspace/"} {
				input, event := lifecyclePayload(t, agent.Name, false, repo)
				input = lifecycleAbsoluteWrite(t, input, repo)
				// Remote URIs deliberately carry the exact local drive/path suffix.
				remote := prefix + filepath.ToSlash(repo)
				input = lifecycleMapStrings(input, func(value string) string {
					value = strings.ReplaceAll(value, filepath.ToSlash(repo), remote)
					return strings.ReplaceAll(value, repo, remote)
				}).(map[string]any)
				lifecycleHook(t, agent.Name, input, event)
				lifecycleNoTracking(t)
			}
			// Bridge and Cursor have explicit locality metadata: keep matching
			// native paths while withdrawing the local-executor assertion.
			input, event := lifecyclePayload(t, agent.Name, false, repo)
			input = lifecycleAbsoluteWrite(t, input, repo)
			if _, bridge := input["version"]; bridge {
				input["local"] = false
				lifecycleHook(t, agent.Name, input, event)
				lifecycleNoTracking(t)
			} else if agent.Name == "cursor" {
				input["is_background_agent"] = true
				lifecycleHook(t, agent.Name, input, event)
				lifecycleNoTracking(t)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "local namesake must survive" {
				t.Fatalf("remote callback touched local namesake: %s %v", data, err)
			}
		})
	}
}

func TestAgentsRemoteWorkspaceNativeNamesake(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range hook.Agents() {
		t.Run(agent.Name, func(t *testing.T) {
			_, repo := lifecycleEnvironment(t)
			lifecycleLocalBackend(t)
			path := filepath.Join(repo, "debug.log")
			writeFixture(t, path, "local namesake must survive")
			input, event := lifecyclePayload(t, agent.Name, false, repo)
			input = lifecycleAbsoluteWrite(t, input, repo)
			// An absolute tool path must not override an explicitly remote cwd.
			// This deliberately leaves the local matching file path in the payload.
			input["cwd"] = "vscode-remote://ssh-remote+obit-test/" + filepath.ToSlash(repo)
			input["workspace_roots"] = []string{input["cwd"].(string)}
			input["workspacePaths"] = []string{input["cwd"].(string)}
			lifecycleHook(t, agent.Name, input, event)
			// A queued write would otherwise hide a registration until replay.
			lifecycleDrainQueue(t)
			lifecycleNoTracking(t)
		})
	}
}

func lifecycleLocalBackend(t *testing.T) {
	t.Helper()
	t.Setenv("OBIT_DEEPSEEK_LOCAL", "1")
	t.Setenv("OBIT_DEEPSEEK_SHELL", "bash")
}

func lifecycleHook(t *testing.T, agent string, input map[string]any, event string) string {
	t.Helper()
	args := []string{"hook", agent}
	if event != "" {
		args = append(args, "--event", event)
	}
	return lifecycleCLI(t, lifecycleJSON(t, input), 0, args...)
}

// The isolated runner from commandRepo can hold store.lock during its first
// tick; the hook then queues the write by design. Replay it before asserting.
func lifecycleDrainQueue(t *testing.T) {
	t.Helper()
	store := core.Store{Home: os.Getenv("OBIT_HOME")}
	for deadline := time.Now().Add(10 * time.Second); store.HasDeferredObservations(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("queued hook writes were not replayed")
		}
		svc, err := app.Open()
		if err != nil {
			t.Fatal(err)
		}
		if err := app.Engine(svc).ReplayObservations(); err != nil {
			t.Fatal(err)
		}
	}
}

func lifecycleState(t *testing.T) core.State {
	t.Helper()
	state, err := (core.Store{Home: os.Getenv("OBIT_HOME")}).Read()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func lifecycleNoTracking(t *testing.T) {
	t.Helper()
	state := lifecycleState(t)
	if len(state.Items)+len(state.Proposals)+len(state.Burials) != 0 {
		t.Fatalf("remote payload affected local ledger: %+v", state)
	}
}

func lifecycleNeutral(t *testing.T, agent, output string) {
	t.Helper()
	registered, ok := hook.Lookup(agent)
	if !ok {
		t.Fatal("missing adapter", agent)
	}
	want := registered.Adapter.Fallback(hook.Event{Name: "PreToolUse"})
	if strings.TrimSpace(output) != strings.TrimSpace(string(want)) {
		t.Fatalf("unsupported/undeclared deletion must pass unchanged: %s", output)
	}
}

func lifecycleDenial(t *testing.T, output string) string {
	t.Helper()
	var reply struct {
		Deny           bool   `json:"deny"`
		Decision       string `json:"permissionDecision"`
		Reason         string `json:"reason"`
		DecisionReason string `json:"permissionDecisionReason"`
		Hook           struct {
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(output), &reply); err != nil {
		t.Fatalf("invalid denial: %s %v", output, err)
	}
	if reply.Hook.Decision == "deny" {
		return reply.Hook.Reason
	}
	if reply.Decision == "deny" {
		return reply.DecisionReason
	}
	if reply.Deny {
		return reply.Reason
	}
	t.Fatalf("host protocol did not deny deletion: %s", output)
	return ""
}

func lifecycleBash(t *testing.T) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(filepath.Dir(filepath.Dir(git)), "bin", "bash.exe"), `C:\Program Files\Git\bin\bash.exe`} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Fatal("Git Bash is required to execute the exact hook suggestion")
	return ""
}
