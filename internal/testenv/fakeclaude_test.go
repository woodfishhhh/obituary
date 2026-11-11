package testenv_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestFakeClaudeReusesTestBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	exe, log := testenv.FakeClaude(t)
	// Only the copied test binary is on PATH. Dispatch happens before TestMain
	// isolation so the fake sees exactly the caller's configuration and log.
	t.Setenv("PATH", filepath.Dir(exe))
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	for _, native := range []string{exe, filepath.Join(t.TempDir(), "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")} {
		if native != exe {
			testenv.FakeClaudeAt(t, native)
		}
		out, err := exec.Command(native, "--version").CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != "2.1.318 (Claude Code)" {
			t.Fatalf("TestMain fake dispatch: %s %v", out, err)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil || strings.Count(string(data), "--version") != 2 {
		t.Fatalf("fake lost caller log: %s %v", data, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("version probe wrote configuration: %v %v", entries, err)
	}
}
