package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestAgentShellDeclarationRejectsRemote(t *testing.T) {
	for _, name := range []string{"codebuddy", "codebuddy-ide", "minimax-code"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("OBIT_CONFIG_DIR", dir)
			if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[agent_shells]\n"+name+" = 'bash'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name, field string
				value       any
				toolInput   bool
			}{
				{"ssh-cwd", "cwd", "ssh://host/repo", false},
				{"cloud-cwd", "cwd", "/workspace/repo", false},
				{"mounted-cwd", "cwd", "/mnt/c/repo", false},
				{"relative-cwd", "cwd", "repo", false},
				{"unc-cwd", "cwd", `\\server\share\repo`, false},
				{"remote-flag", "remote", true, false},
				{"not-local", "local", false, false},
				{"ssh-backend", "backend", "ssh", false},
				{"cloud-backend", "execution_backend", "cloud", false},
				{"environment-id", "environment_id", "remote", false},
				{"tool-remote", "remote", true, true},
				{"tool-not-local", "local", false, true},
				{"tool-backend", "backend", "ssh", true},
				{"tool-cloud", "execution_backend", "cloud", true},
				{"tool-environment", "environment_id", "remote", true},
				{"tool-workdir", "workdir", "/mnt/c/repo", true},
				{"tool-cwd", "cwd", "ssh://host/repo", true},
				{"remote-command", "command", "ssh host rm -rf scratch", true},
				{"remote-tool", "tool_name", "remote_exec", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					input := map[string]any{"command": "rm -rf scratch", "shell": "bash", "local": true}
					payload := map[string]any{"hook_event_name": "PreToolUse", "cwd": `C:\repo`, "tool_name": "Bash", "tool_input": input}
					if tc.toolInput {
						input[tc.field] = tc.value
					} else {
						payload[tc.field] = tc.value
					}
					agent, _ := Lookup(name)
					ev := cGroupDecode(t, agent, payload, "")
					if ev.Cwd != payload["cwd"] || len(ev.Paths) != 0 {
						t.Fatalf("remote namespace mapped: %+v", ev)
					}
					e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
					var out bytes.Buffer
					RunEvent(agent, e, ev, &out, nil)
					if out.Len() != 0 {
						t.Fatalf("remote command intercepted: %s", out.String())
					}
					if _, err := os.Stat(filepath.Join(e.Svc.Store.Home, "store.json")); !os.IsNotExist(err) {
						t.Fatalf("pre opened ledger: %v", err)
					}
				})
			}
		})
	}
}

func TestAgentShellDeclarationsStayScoped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[agent_shells]\ncodebuddy = 'bash'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codebuddy-ide", "minimax-code", "codex"} {
		if got := guardSuggestion(t, name, `C:\repo`, "Bash", map[string]any{"command": "rm -rf scratch"}); got != "" {
			t.Fatalf("codebuddy declaration affected %s: %s", name, got)
		}
	}
}
