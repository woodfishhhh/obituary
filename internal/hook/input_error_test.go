package hook

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestInvalidInputLogIncludesEventAndRedactedPrefix(t *testing.T) {
	for _, tc := range []struct {
		name, input, invocation, event, prefix string
	}{
		{"unescaped-backslash", `{"hook_event_name":"Stop","cwd":"D:\Coding\repo"}`, "", "Stop", `D:\Coding\repo`},
		{"windows", `{"hook_event_name":"Stop","cwd":"C:\Users\private-user\Coding\repo"}`, "", "Stop", `C:\Users\<user>\Coding\repo`},
		{"event-after-error", `{"cwd":"C:\Users\private-user\x","hook_event_name":"SessionEnd"}`, "", "SessionEnd", `C:\Users\<user>\x`},
		{"escaped-windows", `{"cwd":"C:\\Users\\private-user\\repo",}`, "Stop", "Stop", `C:\\Users\\<user>\\repo`},
		{"wsl", `{"cwd":"/mnt/c/Users/private-user/repo",}`, "", "unknown", `/mnt/c/Users/<user>/repo`},
		{"posix", `{"cwd":"/home/private-user/repo",}`, "", "unknown", `/home/<user>/repo`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			var object any
			err := json.Unmarshal([]byte(tc.input), &object)
			if err == nil {
				t.Fatal("fixture must be invalid JSON")
			}
			LogInputError(home, Invocation{Agent: "claude", Event: tc.invocation}, []byte(tc.input), err)
			data, err := os.ReadFile(filepath.Join(home, "hook.log"))
			if err != nil {
				t.Fatal(err)
			}
			line := string(data)
			if !strings.Contains(line, "claude: input:") || !strings.Contains(line, "event_hint="+strconv.Quote(tc.event)) || strings.Contains(line, "private-user") {
				t.Fatalf("host, event, or username redaction incorrect: %s", line)
			}
			_, quoted, _ := strings.Cut(line, "input_prefix=")
			prefix, err := strconv.Unquote(strings.TrimSpace(quoted))
			if err != nil || !strings.Contains(prefix, tc.prefix) {
				t.Fatalf("prefix changed path beyond its username: %q %v", prefix, err)
			}
		})
	}
}

func TestInvalidInputLogBoundsAndQuotesUntrustedText(t *testing.T) {
	home := t.TempDir()
	input := "bad\njson " + strings.Repeat("x", 80) + "do-not-log-this-tail"
	LogInputError(home, Invocation{Agent: "codex"}, []byte(input), errors.New("invalid JSON"))
	data, err := os.ReadFile(filepath.Join(home, "hook.log"))
	if err != nil {
		t.Fatal(err)
	}
	_, quoted, _ := strings.Cut(string(data), "input_prefix=")
	prefix, err := strconv.Unquote(strings.TrimSpace(quoted))
	if err != nil || prefix != input[:80] || strings.Count(string(data), "\n") != 1 || strings.Contains(string(data), "do-not-log-this-tail") {
		t.Fatalf("unbounded or multi-line log: %s %v", data, err)
	}
}

func TestValidInputFailureDoesNotLogPayloadContents(t *testing.T) {
	home := t.TempDir()
	LogInputError(home, Invocation{Agent: "codex"}, []byte(`{"content":"private-file-content"}`), errors.New("unsupported event"))
	data, err := os.ReadFile(filepath.Join(home, "hook.log"))
	if err != nil || strings.Contains(string(data), "private-file-content") || strings.Contains(string(data), "input_prefix=") {
		t.Fatalf("valid input contents leaked: %s %v", data, err)
	}
}
