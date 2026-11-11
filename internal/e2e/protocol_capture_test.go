//go:build windows

package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in only: consumes an authenticated agent invocation. No credentials or
// user settings are copied. A local fixture test cannot certify agent behavior.
func TestCaptureClaudeDeleteGuardProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	if os.Getenv("OBIT_CAPTURE_CLAUDE") != "1" {
		t.Skip("opt-in real-agent deny protocol recording")
	}
	source, err := filepath.Abs("testdata/capture/main.go")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, name := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, dir)
	}
	helper := filepath.Join(root, "capture.exe")
	if out, err := exec.Command("go", "build", "-o", helper, source).CombinedOutput(); err != nil {
		t.Fatalf("capture build: %s %v", out, err)
	}
	repo, _ := commandRepo(t)
	capture := filepath.Join(repo, "payload.json")
	command := `"` + filepath.ToSlash(helper) + `" "` + filepath.ToSlash(capture) + `"`
	settings := map[string]any{"hooks": map[string]any{"PreToolUse": []any{map[string]any{"matcher": "Bash", "hooks": []any{map[string]string{"type": "command", "command": command}}}}}}
	data, _ := json.Marshal(settings)
	writeFixture(t, filepath.Join(repo, ".claude", "settings.json"), string(data))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	args := []string{"-c", `exec claude --setting-sources project --strict-mcp-config --mcp-config '{"mcpServers":{}}' --tools Bash --allowedTools Bash --model haiku --max-turns 3 --output-format stream-json --verbose -p 'Use Bash exactly once to run: echo OBIT_PROTOCOL_EXECUTED > executed.txt . The final period is punctuation. If the call is denied, quote the exact denial reason in your final answer and do not retry.'`}
	out, err := exec.CommandContext(ctx, `C:\Program Files\Git\bin\bash.exe`, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("agent: %v %s", err, out)
	}
	payload, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("hook not invoked: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(repo, "executed.txt")); !os.IsNotExist(err) {
		t.Fatalf("denied command executed: %v", err)
	}
	seen := false
	for _, line := range strings.Split(string(out), "\n") {
		var event struct {
			Type   string `json:"type"`
			Result string `json:"result"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Type == "result" && strings.Contains(event.Result, "OBIT_DENY_REASON_VISIBLE") {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("agent did not report seeing the deny reason: %s", out)
	}
	var input map[string]any
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"cwd", "transcript_path", "session_id", "tool_use_id", "prompt_id"} {
		if _, ok := input[key]; ok {
			input[key] = "redacted"
		}
	}
	sample, _ := json.MarshalIndent(input, "", "  ")
	t.Logf("deny prevented execution; reason visible in final answer; input:\n%s", sample)
}
