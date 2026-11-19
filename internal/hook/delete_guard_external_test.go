package hook_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
)

func TestPOSIXNativeShellDeletionGuard(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native POSIX executable; Windows WSL interop has separate coverage")
	}
	for _, distro := range []string{"", "Ubuntu"} {
		t.Run("WSL_DISTRO_NAME="+distro, func(t *testing.T) {
			t.Setenv("WSL_DISTRO_NAME", distro)
			t.Setenv("OBIT_GUARD_RM", "1")
			t.Setenv("PATH", "")
			for _, agent := range []string{"claude", "codex"} {
				root := t.TempDir()
				path := filepath.Join(root, "victim")
				if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
				engine := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}}}
				input, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "cwd": root, "tool_name": "Bash", "tool_input": map[string]string{"command": "rm -rf -- victim", "shell": "bash"}})
				var output bytes.Buffer
				hook.Run(agent, engine, bytes.NewReader(input), &output, nil)
				if !bytes.Contains(output.Bytes(), []byte(`"permissionDecision":"deny"`)) || !bytes.Contains(output.Bytes(), []byte(" rm -rf -- victim")) {
					t.Fatalf("native %s deletion was not denied: %s", agent, output.String())
				}
				if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
					t.Fatalf("pre-tool guard changed the operand: %q %v", data, err)
				}
			}
		})
	}
}

func TestCodexEmbeddedDeletionGuardScope(t *testing.T) {
	t.Setenv("OBIT_GUARD_RM", "1")
	t.Setenv("OBIT_DISABLED", "0")
	incident, err := os.ReadFile(filepath.Join("testdata", "codex-windows-incident-command.txt"))
	if err != nil {
		t.Fatal(err)
	}
	command := string(incident)
	cases := []struct {
		name, agent, tool, key string
		cwd                    string
		wantDenied             bool
	}{
		{"codex-windows", "codex", "exec_command", "cmd", "C:/repo", runtime.GOOS == "windows"},
		{"claude-unchanged", "claude", "Bash", "command", "C:/repo", false},
		// On Windows a POSIX cwd skips the ordinary guard (WSL context); this rule still applies.
		{"codex-posix-cwd", "codex", "exec_command", "cmd", "/tmp/repo", runtime.GOOS == "windows"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := map[string]any{tc.key: command}
			payload, err := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"cwd":             tc.cwd,
				"tool_name":       tc.tool,
				"tool_input":      input,
			})
			if err != nil {
				t.Fatal(err)
			}
			home := t.TempDir()
			engine := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: home}}}
			var output bytes.Buffer
			hook.Run(tc.agent, engine, bytes.NewReader(payload), &output, nil)
			denied := bytes.Contains(output.Bytes(), []byte("\"permissionDecision\":\"deny\""))
			if denied != tc.wantDenied {
				t.Fatalf("embedded deletion denial: got %v, want %v; output=%s", denied, tc.wantDenied, output.String())
			}
			if tc.agent == "codex" && tc.wantDenied {
				var response map[string]map[string]string
				if err := json.Unmarshal(output.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				reason := response["hookSpecificOutput"]["permissionDecisionReason"]
				for _, phrase := range []string{
					"Codex on Windows passes commands through another shell",
					"quoting may fail",
					"may really execute",
					"an editing tool writes it to a file",
					"the command receives the file path",
				} {
					if !strings.Contains(reason, phrase) {
						t.Errorf("denial reason lacks %q: %s", phrase, reason)
					}
				}
				audit, err := os.ReadFile(filepath.Join(home, "delete-guard.jsonl"))
				if err != nil || !strings.Contains(string(audit), "\"kind\":\"denied\"") {
					t.Fatalf("embedded deletion denial was not audited: %s %v", audit, err)
				}
			}
		})
	}
}

func TestCodexEmbeddedDeletionCases(t *testing.T) {
	t.Setenv("OBIT_GUARD_RM", "1")
	t.Setenv("OBIT_DISABLED", "0")
	cases := []struct {
		name, file string
		wantDenied bool
	}{
		{"backtick-rm", "codex-embedded-rm.txt", true},
		{"dollar-find", "codex-embedded-find.txt", true},
		{"heredoc-xargs", "codex-embedded-xargs.txt", true},
		{"backtick-text", "codex-embedded-text.txt", false},
		{"backtick-obit", "codex-embedded-obit.txt", false},
		{"backtick-go", "codex-embedded-go.txt", false},
		{"backtick-ls", "codex-embedded-ls.txt", false},
		{"ordinary-command", "codex-embedded-ordinary.txt", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"cwd":             "C:/repo",
				"tool_name":       "exec_command",
				"tool_input":      map[string]any{"cmd": string(command)},
			})
			if err != nil {
				t.Fatal(err)
			}
			e := &auto.Engine{Svc: &core.Service{Store: core.Store{Home: t.TempDir()}}}
			var output bytes.Buffer
			hook.Run("codex", e, bytes.NewReader(payload), &output, nil)
			denied := bytes.Contains(output.Bytes(), []byte("\"permissionDecision\":\"deny\""))
			want := runtime.GOOS == "windows" && tc.wantDenied
			if denied != want {
				t.Fatalf("embedded case denial: got %v, want %v; output=%s", denied, want, output.String())
			}
		})
	}
}
