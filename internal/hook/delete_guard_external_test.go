package hook_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
