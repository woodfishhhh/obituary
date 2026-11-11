//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

// Run the built CLI with disposable host settings and assert stdout plus exit
// status to verify the installation lifecycle without touching user settings.
func TestAgentsInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range integrate.Agents() {
		for _, global := range []bool{false, true} {
			scope := "project"
			if global {
				scope = "global"
			}
			t.Run(agent.Name+"/"+scope, func(t *testing.T) {
				root, repo := lifecycleEnvironment(t)
				if agent.Name == "claude" && global {
					// TestMain doubles as the official CLI stub; the real client is never run.
					testenv.FakeClaude(t)
				}
				layout := lifecycleLayout(t, agent.Name, root, repo, global)
				originals := lifecycleSeed(t, layout)
				args := []string{"init", "--agent", agent.Name, "--yes"}
				if global {
					args = append(args, "--global")
				}
				if agent.Capabilities.MCP.Level != hook.Unsupported {
					args = append(args, "--mcp")
				}
				if agent.Capabilities.Skill.Level != hook.Unsupported {
					args = append(args, "--skill")
				}
				if agent.Capabilities.DeleteGuard.Level != hook.Unsupported {
					// These limited adapters explicitly lack executable-shell evidence.
					if slices.Contains([]string{"qoder", "qoder-cn", "lingma", "qoderwork", "zcode"}, agent.Name) || agent.Name == "hermes" && !global {
						before := lifecycleFiles(t, root, repo)
						out := lifecycleCLI(t, nil, 1, append(args, "--guard-rm")...)
						if !strings.Contains(out, "guard") || !strings.Contains(out, "shell") && !strings.Contains(out, "wrapper") && !strings.Contains(out, "--global") {
							t.Fatalf("limited guard prerequisites missing: %s", out)
						}
						lifecycleUnchanged(t, before, lifecycleFiles(t, root, repo))
					} else {
						args = append(args, "--guard-rm")
					}
				}
				before := lifecycleFiles(t, root, repo)
				lifecycleCLI(t, nil, layout.exit, append(args, "--dry-run")...)
				lifecycleUnchanged(t, before, lifecycleFiles(t, root, repo))
				out := lifecycleCLI(t, nil, layout.exit, args...)
				for _, step := range layout.steps {
					if !strings.Contains(out, step) {
						t.Errorf("missing host setup step %q: %s", step, out)
					}
				}
				for _, path := range layout.files {
					data, err := os.ReadFile(path)
					if err != nil || !bytes.Contains(bytes.ToLower(data), []byte("obit")) {
						t.Errorf("host discovery target missing Obituary: %s: %s %v", path, data, err)
					}
				}
				for _, path := range layout.absent {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Errorf("installer guessed unsupported target %s: %v", path, err)
					}
				}
				installed := lifecycleFiles(t, root, repo)
				modified := lifecycleModTimes(t, root, repo)
				lifecycleCLI(t, nil, layout.exit, args...)
				lifecycleUnchanged(t, installed, lifecycleFiles(t, root, repo))
				if !reflect.DeepEqual(modified, lifecycleModTimes(t, root, repo)) {
					t.Error("repeated installation rewrote identical bytes")
				}
				lifecycleCLI(t, nil, 0, append(args, "--show")...)
				lifecycleUnchanged(t, installed, lifecycleFiles(t, root, repo))
				out = lifecycleCLI(t, nil, layout.exit, append(args, "--uninstall")...)
				for path, original := range originals {
					data, err := os.ReadFile(path)
					if err != nil || !bytes.Contains(data, []byte("user-sentinel")) {
						t.Errorf("user configuration lost: %s: %s %v", path, data, err)
					}
					if path == layout.sentinel && string(data) != original {
						t.Errorf("unrelated user bytes changed: %s", path)
					}
					lifecycleUserConfig(t, path, original, string(data))
				}
				if agent.Name == "openclaw" {
					for _, step := range []string{"disable plugins.entries.obituary", "plugins.allow", "plugins.load.paths", "retained"} {
						if !strings.Contains(out, step) {
							t.Errorf("missing safe manual unregistration step %q: %s", step, out)
						}
					}
					lifecycleUnchanged(t, installed, lifecycleFiles(t, root, repo))
					t.Log("OpenClaw retains its bundle until manual Gateway deactivation is verified")
				} else {
					for path, data := range lifecycleFiles(t, root, repo) {
						name := filepath.Base(path)
						if strings.HasSuffix(name, ".bak") || strings.Contains(name, ".bak.") {
							continue // Installation retains numbered recovery backups for the user.
						}
						if strings.Contains(strings.ToLower(data), "obituary") || strings.Contains(data, "obit:") || strings.Contains(data, "hook "+agent.Name) {
							t.Errorf("active Obituary content remains after uninstall: %s: %s", path, data)
						}
					}
				}
				removed := lifecycleFiles(t, root, repo)
				lifecycleCLI(t, nil, layout.exit, append(args, "--uninstall")...)
				lifecycleUnchanged(t, removed, lifecycleFiles(t, root, repo))
			})
		}
	}
	for _, agent := range hook.Agents() {
		if _, ok := integrate.LookupAgent(agent.Name); !ok {
			t.Errorf("registered hook %s has no installation lifecycle coverage", agent.Name)
		}
	}
}

func lifecycleEnvironment(t *testing.T) (root, repo string) {
	t.Helper()
	root = t.TempDir()
	for key, name := range map[string]string{
		"OBIT_USER_HOME": "user", "CLAUDE_CONFIG_DIR": "claude", "CODEX_HOME": "codex",
		"COPILOT_HOME": "copilot", "GEMINI_CLI_HOME": "gemini", "FACTORY_HOME_OVERRIDE": "factory",
		"VIBE_HOME": "vibe", "KIMI_SHARE_DIR": "kimi", "DSH_HOME": "deepseek",
		"QODER_CONFIG_DIR": "qoder", "QODERCN_CONFIG_DIR": "qoder-cn", "CODEBUDDY_CONFIG_DIR": "codebuddy",
		"CLINE_DIR": "cline", "CLINE_DATA_DIR": "cline-data", "HERMES_HOME": "hermes", "KILO_CONFIG_DIR": "kilo",
		"OPENCODE_CONFIG_DIR": "opencode", "OPENCLAW_STATE_DIR": "openclaw", "OPENCLAW_HOME": "openclaw-home",
		"PI_CODING_AGENT_DIR": "pi", "XDG_CONFIG_HOME": "xdg", "MINIMAX_DATA_DIR": "minimax",
		"MAVIS_DATA_DIR": "mavis", "MCODE_INSTALL_DIR": "minimax-install", "ZCODE_DATA_BASE_DIR": "zcode-data",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, path)
	}
	for _, key := range []string{"OMP_PROFILE", "PI_PROFILE", "PI_CONFIG_DIR", "OPENCLAW_PROFILE", "OPENCLAW_CONFIG_PATH", "OPENCODE_CONFIG", "OBIT_SESSION_ID", "CLAUDE_CODE_SESSION_ID", "CODEX_THREAD_ID", "BASH_ENV"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OBIT_DISABLED", "0")
	t.Setenv("OBIT_GUARD_RM", "1")
	t.Setenv("OBIT_HOOK_AUDIT", "0")
	repo, _ = commandRepo(t)
	repo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	return root, repo
}

func lifecycleCLI(t *testing.T, input []byte, code int, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath(t, "obit"), args...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	actual := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		actual = exit.ExitCode()
	}
	if actual != code {
		t.Fatalf("obit %q exit=%d, want %d; stdout=%s stderr=%s", args, actual, code, &stdout, &stderr)
	}
	if len(args) > 0 && args[0] == "hook" && stderr.Len() != 0 {
		t.Fatalf("hook wrote stderr: %s", &stderr)
	}
	return stdout.String() + stderr.String()
}

func lifecycleFiles(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, root := range roots {
		for rel, entry := range demoTree(t, root) {
			if !entry.Directory && rel != ".git" && !strings.HasPrefix(rel, ".git/") {
				files[filepath.Join(root, rel)] = entry.Data
			}
		}
	}
	return files
}

func lifecycleUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		for path, data := range after {
			if original, exists := before[path]; !exists || data != original {
				t.Errorf("unexpected file change: %s", path)
			}
		}
		for path := range before {
			if _, exists := after[path]; !exists {
				t.Errorf("unexpected file removal: %s", path)
			}
		}
	}
}

func lifecycleUserConfig(t *testing.T, path, before, after string) {
	t.Helper()
	if json.Valid([]byte(before)) {
		var original, current map[string]any
		_ = json.Unmarshal([]byte(before), &original)
		if err := json.Unmarshal([]byte(after), &current); err != nil {
			t.Fatalf("user JSON corrupted at %s: %v", path, err)
		}
		for key, value := range original {
			if !reflect.DeepEqual(value, current[key]) {
				t.Errorf("user JSON member changed at %s/%s: got %#v, want %#v", path, key, current[key], value)
			}
		}
	} else if before != after {
		t.Errorf("user configuration bytes changed at %s: got %q, want %q", path, after, before)
	}
}

func lifecycleModTimes(t *testing.T, roots ...string) map[string]int64 {
	t.Helper()
	times := map[string]int64{}
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Name() == ".git" && entry.IsDir() {
				return filepath.SkipDir
			}
			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				times[path] = info.ModTime().UnixNano()
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return times
}

func lifecycleJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
