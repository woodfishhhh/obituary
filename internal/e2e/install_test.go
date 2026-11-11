//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardInstalledExecutableReference(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	self := binaryPath(t, "obit")
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "bin 中文 'quoted'", "obit.exe")
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, data, 0o700); err != nil {
		t.Fatal(err)
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing", "other", "current"} {
		t.Run(mode, func(t *testing.T) {
			repo, store := commandRepo(t)
			writeFixture(t, filepath.Join(repo, "victim"), "restore me")
			searchPath := ""
			switch mode {
			case "other":
				searchPath = t.TempDir()
				writeFixture(t, filepath.Join(searchPath, "obit.exe"), "not the installed executable")
			case "current":
				searchPath = filepath.Dir(binary)
			}
			env := append(os.Environ(), "OBIT_DISABLED=0", "OBIT_GUARD_RM=1", "PATH="+searchPath)
			for _, agent := range []string{"claude", "codex"} {
				install := exec.Command(binary, "init", "--agent", agent, "--yes", "--guard-rm")
				install.Env = env
				if out, err := install.CombinedOutput(); err != nil {
					t.Fatalf("isolated project install: %s %v", out, err)
				}
				file := filepath.Join(repo, ".claude", "settings.json")
				if agent == "codex" {
					file = filepath.Join(repo, ".codex", "hooks.json")
				}
				data, err := os.ReadFile(file)
				var settings struct {
					Hooks map[string][]struct {
						Hooks []struct {
							Command, CommandWindows string
							Args                    []string
						}
					}
				}
				if err != nil || json.Unmarshal(data, &settings) != nil || len(settings.Hooks["PreToolUse"]) != 1 {
					t.Fatalf("installed handler missing: %s %v", data, err)
				}
				handler := settings.Hooks["PreToolUse"][0].Hooks[0]
				installed := handler.Command
				if agent == "codex" {
					installed = handler.CommandWindows
				}
				if mode != "current" && !strings.Contains(installed, filepath.ToSlash(binary)) {
					t.Fatalf("hook did not pin its executable: %s", installed)
				}
				for _, shell := range []string{"bash", "powershell", "wsl"} {
					if agent == "claude" && shell != "bash" {
						continue
					}
					command, cwd := "rm -f victim", repo
					tool := "Bash"
					input := map[string]string{"command": command, "shell": "bash"}
					want := "'" + strings.ReplaceAll(filepath.ToSlash(binary), "'", `'\''`) + "' " + command
					switch shell {
					case "powershell":
						input = map[string]string{"cmd": `Remove-Item -Force .\victim`, "shell": "pwsh"}
						tool = "exec_command"
						want = "& '" + strings.ReplaceAll(binary, "'", "''") + `' remove-item -Force .\victim`
					case "wsl":
						cwd, tool = "/mnt/c/work", "exec_command"
						input = map[string]string{"cmd": command}
						interop := "/mnt/" + strings.ToLower(binary[:1]) + filepath.ToSlash(binary[2:])
						want = "'" + strings.ReplaceAll(interop, "'", `'\''`) + "' " + command
					}
					if mode == "current" && shell != "wsl" {
						want = "obit " + command
						if shell == "powershell" {
							want = `obit remove-item -Force .\victim`
						}
					}
					payload, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": cwd, "tool_name": tool, "tool_input": input})
					child := exec.Command(`C:\Program Files\Git\bin\bash.exe`, "--noprofile", "--norc", "-c", installed)
					if agent == "claude" {
						if !filepath.IsAbs(installed) || strings.Join(handler.Args, " ") != "hook claude" {
							t.Fatalf("invalid Claude exec form: %+v", handler)
						}
						child = exec.Command(installed, handler.Args...)
					}
					child.Env, child.Stdin = env, bytes.NewReader(payload)
					out, err := child.CombinedOutput()
					var reply struct {
						Hook struct {
							Decision string `json:"permissionDecision"`
							Reason   string `json:"permissionDecisionReason"`
						} `json:"hookSpecificOutput"`
					}
					if err != nil || json.Unmarshal(out, &reply) != nil || reply.Hook.Decision != "deny" {
						t.Fatalf("installed hook (%s/%s): %s %v", agent, shell, out, err)
					}
					parts := strings.Split(reply.Hook.Reason, "`")
					if len(parts) < 3 || parts[1] != want {
						t.Fatalf("suggestion uses wrong executable (%s/%s): %s; want %s", agent, shell, reply.Hook.Reason, want)
					}
					if shell != "bash" {
						continue // quoting checked; no live PowerShell or WSL host is launched
					}
					// Execute the exact suggestion, without replacing its executable.
					child = exec.Command(`C:\Program Files\Git\bin\bash.exe`, "--noprofile", "--norc", "-c", parts[1])
					child.Env = env
					if out, err := child.CombinedOutput(); err != nil {
						t.Fatalf("copied suggestion: %s %v", out, err)
					}
					st, err := store.Read()
					if err != nil || len(st.Burials) != 1 || filepath.Base(st.Burials[0].Path) != "victim" {
						t.Fatalf("wrong burial: %+v %v", st.Burials, err)
					}
					restore := exec.Command(binary, "resurrect", fmt.Sprint(st.Burials[0].ID))
					restore.Env = env
					if out, err := restore.CombinedOutput(); err != nil {
						t.Fatalf("resurrection: %s %v", out, err)
					}
					if data, err := os.ReadFile(filepath.Join(repo, "victim")); err != nil || string(data) != "restore me" {
						t.Fatalf("payload not restored: %s %v", data, err)
					}
				}
			}
		})
	}
}
