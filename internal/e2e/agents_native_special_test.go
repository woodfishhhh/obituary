//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// Executes the commands read back from actual installed native configurations.
// These hosts use JSON/TOML command hooks, so the bundle has no TS/Python shim.
func TestNativeSpecialInstalledHookCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"trae", "trae-cn", "antigravity", "vibe", "kimi"} {
		t.Run(agent, func(t *testing.T) {
			repo, store := commandRepo(t)
			root := t.TempDir()
			t.Setenv("OBIT_USER_HOME", root)
			t.Setenv("KIMI_SHARE_DIR", filepath.Join(root, "kimi"))
			t.Setenv("VIBE_HOME", filepath.Join(root, "vibe"))
			binary := binaryPath(t, "obit")
			if agent != "vibe" {
				data, err := os.ReadFile(binary)
				if err != nil {
					t.Fatal(err)
				}
				binary = filepath.Join(root, "tools with spaces & Unicode 中文", "obit.exe")
				writeFixture(t, binary, string(data))
			}
			s := integrate.Scope{Project: repo, Global: agent == "kimi"}
			plan, err := integrate.PlanAgent(agent, s, integrate.Commands{Native: `"` + filepath.ToSlash(binary) + `" hook ` + agent, Executable: binary}, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := integrate.Apply(plan); err != nil {
				t.Fatal(err)
			}
			var path, event, command string
			payload := map[string]any{"session_id": "installed-session", "cwd": repo}
			switch agent {
			case "trae", "trae-cn":
				path, event = filepath.Join(repo, ".trae", "hooks.json"), "SessionStart"
				var config struct {
					Hooks map[string][]struct{ Hooks []struct{ Command string } }
				}
				data, err := os.ReadFile(path)
				if err != nil || json.Unmarshal(data, &config) != nil {
					t.Fatal("read installed hooks", err)
				}
				command = config.Hooks[event][0].Hooks[0].Command
			case "antigravity":
				path, event = filepath.Join(repo, ".agents", "plugins", "obituary", "hooks.json"), "SessionStart"
				var config map[string]map[string]json.RawMessage
				data, err := os.ReadFile(path)
				if err != nil || json.Unmarshal(data, &config) != nil {
					t.Fatal("read installed bundle", err)
				}
				var handlers []struct{ Command string }
				if err := json.Unmarshal(config["obituary"][event], &handlers); err != nil || len(handlers) != 1 {
					t.Fatal("flat lifecycle hook schema", err)
				}
				command = handlers[0].Command
				payload = map[string]any{"conversationId": "installed-session", "workspacePaths": []string{repo}}
			case "vibe", "kimi":
				path, event = filepath.Join(repo, ".vibe", "hooks.toml"), "post_tool"
				if agent == "kimi" {
					path, event = filepath.Join(root, "kimi", "config.toml"), "Stop"
				}
				var config struct{ Hooks []map[string]any }
				data, err := os.ReadFile(path)
				if err != nil || toml.Unmarshal(data, &config) != nil {
					t.Fatal("read installed TOML", err)
				}
				for _, entry := range config.Hooks {
					if entry["type"] == event || entry["event"] == event {
						command = entry["command"].(string)
					}
				}
				if agent == "vibe" {
					file := filepath.Join(repo, "debug.log")
					writeFixture(t, file, "debug content")
					payload["tool_status"], payload["tool_name"] = "success", "file_system.write_file"
					payload["tool_input"] = map[string]string{"file_path": file}
				}
			}
			payload["hook_event_name"] = event
			raw, _ := json.Marshal(payload)
			if command == "" {
				t.Fatal("empty installed command")
			}
			run := func(input []byte) []byte {
				t.Helper()
				// cmd.exe takes a command string, not CommandLineToArgvW quoting.
				// Match Python create_subprocess_shell's outer quotes explicitly.
				cmd := exec.Command("cmd.exe")
				cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /s /c "` + command + `"`}
				if agent == "trae" || agent == "trae-cn" || agent == "vibe" {
					cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$inputText = [Console]::In.ReadToEnd(); $inputText | "+command+"; exit $LASTEXITCODE")
				}
				cmd.Stdin = bytes.NewReader(input)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Run(); err != nil || stderr.Len() != 0 {
					t.Fatalf("installed %s hook command: %s: %v; %s", agent, command, err, stderr.String())
				}
				return bytes.TrimSpace(stdout.Bytes())
			}
			out := run(raw)
			switch agent {
			case "vibe":
				st, err := store.Read()
				if err != nil || len(st.Proposals) != 1 || !bytes.Contains(out, []byte("obit approve")) {
					t.Fatalf("native write observation/context: %s %+v %v", out, st.Proposals, err)
				}
			case "kimi":
				if len(out) != 0 {
					t.Fatalf("Stop must stay silent: %s", out)
				}
			default:
				files, err := filepath.Glob(filepath.Join(store.Home, "sessions", "*.json"))
				if err != nil || len(files) != 1 {
					t.Fatalf("session command did not reach adapter: %v %v", files, err)
				}
				data, err := os.ReadFile(files[0])
				if err != nil || !strings.Contains(string(data), agent+":installed-session") {
					t.Fatalf("wrong adapter/session: %s %v", data, err)
				}
			}
			out = run([]byte("bad json"))
			if agent == "antigravity" {
				if string(out) != "{}" {
					t.Fatal("native failure response", string(out))
				}
			} else if len(out) != 0 {
				t.Fatal("native failure should be silent", string(out))
			}
		})
	}
}
