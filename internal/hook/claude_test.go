package hook

import (
	"bytes"
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestPOSIXClaudeWriteCreatesProposalAndStopIsSilent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX hook integration")
	}
	if testing.Short() {
		t.Skip("creates a real Git repository")
	}
	e := newEnv(t)
	path := e.write("debug.log")
	e.wrote(path)
	state := e.state()
	if len(state.Items) != 0 || len(state.Proposals) != 1 || len(state.Proposals[0].Candidates) != 1 || state.Proposals[0].Candidates[0].Path != path {
		t.Fatalf("write observation was not proposed: %+v", state)
	}
	for _, name := range []string{"Stop", "SessionEnd"} {
		if got := e.event(map[string]any{"hook_event_name": name, "session_id": "posix-session", "cwd": e.root}); got != "" {
			t.Fatalf("%s was not silent: %s", name, got)
		}
	}
}

func TestClaudePowerShellPayload(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, command, want string }{
		{"literal", `Remove-Item -LiteralPath 'build dir' -Recurse -Force`, `remove-item -LiteralPath 'build dir' -Recurse -Force`},
		{"alias", `ri -LiteralPath build`, `remove-item -LiteralPath build`},
		{"rm-alias", `rm -Recurse -Force build`, `remove-item -Recurse -Force build`},
		{"wildcard", `Remove-Item *.log`, ""},
		{"pipeline", `Get-ChildItem | Remove-Item`, ""},
		{"bash-options", `rm -rf build`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := guardSuggestion(t, "claude", `C:\repo`, "PowerShell", map[string]any{"command": tc.command})
			want := tc.want
			if want != "" {
				want = expectedGuardExecutable(t, guardrm.PowerShell) + " " + want
			}
			if got != want {
				t.Fatalf("PowerShell payload: got %q, want %q", got, want)
			}
		})
	}
}

func BenchmarkClaudeHook(b *testing.B) { benchmarkHook(b, "claude", Claude) }

func FuzzClaudePayload(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		// Decode the same envelope as Claude without dispatching untrusted paths
		// to Handle, which would intentionally observe files and update a ledger.
		var input claudeInput
		if err := json.Unmarshal(data, &input); err != nil {
			return
		}
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		var again claudeInput
		if err := json.Unmarshal(encoded, &again); err != nil || !reflect.DeepEqual(input, again) {
			t.Fatalf("Claude envelope changed on round trip: %v", err)
		}
		for _, keys := range [][]string{{"file_path", "notebook_path"}, {"command"}} {
			got := firstString(input.ToolInput, keys...)
			if got != firstString(again.ToolInput, keys...) {
				t.Fatal("Claude tool input changed on round trip")
			}
			if got != "" {
				found := false
				for _, key := range keys {
					found = found || input.ToolInput[key] == got
				}
				if !found {
					t.Fatal("non-string tool input was coerced into a path or command")
				}
			}
		}
	})
}

func TestClaudeNormalizesLongSession(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	var sessions []string
	runner := &Runner{Finish: func(_, session string, _ bool) { sessions = append(sessions, session) }}
	for range 2 {
		payload, err := json.Marshal(map[string]any{"hook_event_name": "SessionEnd", "session_id": strings.Repeat("s", 4097), "cwd": e.root})
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		Claude(e.engine, bytes.NewReader(payload), &out, runner)
	}
	if len(sessions) != 2 || len(sessions[0]) != 71 || sessions[0] != sessions[1] || !strings.HasPrefix(sessions[0], "sha256:") {
		t.Fatalf("unstable or oversized Claude session: %v", sessions)
	}
}
