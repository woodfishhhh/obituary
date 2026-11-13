package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestCopilotGuardRequiresNativeShellEvidence(t *testing.T) {
	t.Parallel()
	agent, ok := Lookup("copilot")
	if !ok {
		t.Fatal("Copilot adapter is not registered")
	}
	for _, tc := range []struct {
		name, event, input string
		shell              guardrm.Shell
	}{
		{"native-powershell", "preToolUse", testenv.NativeFixture(`{"cwd":"C:/repo","toolName":"powershell","toolArgs":{"command":"Remove-Item a.log"}}`), guardrm.PowerShell},
		{"native-bash", "preToolUse", testenv.NativeFixture(`{"cwd":"C:/repo","toolName":"bash","toolArgs":"{\"command\":\"rm a.log\"}"}`), guardrm.Bash},
		{"compat-bash-is-ambiguous", "", testenv.NativeFixture(`{"hook_event_name":"PreToolUse","cwd":"C:/repo","tool_name":"Bash","tool_input":{"command":"rm a.log","shell":"bash"}}`), ""},
		{"vscode-terminal-is-ambiguous", "", testenv.NativeFixture(`{"hook_event_name":"PreToolUse","cwd":"C:/repo","tool_name":"run_in_terminal","tool_input":{"command":"rm a.log"}}`), ""},
		{"remote-filesystem", "preToolUse", `{"cwd":"/workspace/repo","toolName":"bash","toolArgs":{"command":"rm a.log"}}`, ""},
		{"code-mode", "preToolUse", testenv.NativeFixture(`{"cwd":"C:/repo","toolName":"code","toolArgs":{"command":"rm a.log"}}`), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS != "windows" && tc.name == "remote-filesystem" {
				tc.shell = guardrm.Bash // This is a native absolute directory on POSIX.
			}
			ev, err := Decode(agent.Adapter, []byte(tc.input), Invocation{Agent: "copilot", Event: tc.event})
			if err != nil {
				t.Fatal(err)
			}
			if tc.shell == "" {
				if ev.SkipReason == "" {
					t.Fatalf("unverified shell accepted: %+v", ev)
				}
			} else if ev.Shell != tc.shell || ev.SkipReason != "" {
				t.Fatalf("native shell lost: %+v", ev)
			}
		})
	}
}

func copilotFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(testenv.NativeFixture(string(raw))), &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func TestCopilotDocumentedEnvelopes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file, host, event, session string
		paths                      int
	}{
		{"copilot-cli-pre", "preToolUse", "PreToolUse", "copilot:cli-1", 0},
		{"copilot-cli-post", "postToolUse", "PostToolUse", "copilot:cli-1", 1},
		{"copilot-vscode-pre", "", "PreToolUse", "copilot:vscode-1", 0},
		{"copilot-vscode-post", "", "PostToolUse", "copilot:vscode-1", 1},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			raw, _ := json.Marshal(copilotFixture(t, tc.file))
			for _, prefix := range []string{"", "\xef\xbb\xbf"} {
				ev, err := Decode(copilotAdapter{}, append([]byte(prefix), raw...), Invocation{Agent: "copilot", Event: tc.host})
				if err != nil || ev.Name != tc.event || ev.Session != tc.session || len(ev.Paths) != tc.paths {
					t.Fatalf("decoded %+v, %v", ev, err)
				}
			}
		})
	}
}

func TestCopilotRejectsMalformedAndConflictingEnvelopes(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "{", "[]", "null", `{"toolArgs":7}`, `{"toolArgs":"broken"}`, `{"hook_event_name":"PreToolUse","toolName":"bash"}`} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Decode(copilotAdapter{}, []byte(raw), Invocation{Agent: "copilot", Event: "preToolUse"}); err == nil {
				t.Fatal("invalid envelope accepted")
			}
		})
	}
	for _, host := range []string{"unknown", "postToolUseFailure", "SubagentStop"} {
		if _, err := Decode(copilotAdapter{}, []byte(`{}`), Invocation{Agent: "copilot", Event: host}); !errors.Is(err, ErrSkip) {
			t.Fatalf("%s: %v", host, err)
		}
	}
}

func TestCopilotFailedAndUnverifiedWritesDoNotObserve(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"failed", func(m map[string]any) { m["toolResult"] = map[string]any{"resultType": "failure"} }},
		{"missing-result", func(m map[string]any) { delete(m, "toolResult") }},
		{"missing-session", func(m map[string]any) { delete(m, "sessionId") }},
		{"remote", func(m map[string]any) { m["cwd"] = testenv.ForeignFixture("/workspace/repo") }},
		{"remote-file", func(m map[string]any) {
			m["toolArgs"] = map[string]any{"path": testenv.ForeignFixture("/remote/file.log")}
		}},
		{"code-mode", func(m map[string]any) { m["toolName"] = "code" }},
		{"terminal-path", func(m map[string]any) {
			m["toolName"] = "powershell"
			m["toolArgs"] = map[string]any{"path": testenv.NativeFixture("C:/repo/debug.log"), "command": "echo file_path"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := copilotFixture(t, "copilot-cli-post")
			tc.change(in)
			raw, _ := json.Marshal(in)
			if ev, err := Decode(copilotAdapter{}, raw, Invocation{Agent: "copilot", Event: "postToolUse"}); !errors.Is(err, ErrSkip) || len(ev.Paths) != 0 {
				t.Fatalf("write accepted: %+v %v", ev, err)
			}
		})
	}
}

func TestCopilotWritesCreateRealProjectProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, compat := range []bool{false, true} {
		name := "copilot-cli-post"
		host := "postToolUse"
		if compat {
			name = "copilot-vscode-post"
			host = ""
		}
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			path := e.write("debug.log")
			in := copilotFixture(t, name)
			in["cwd"] = e.root
			key := "toolArgs"
			if compat {
				key = "tool_input"
			}
			in[key] = map[string]any{"path": path, "file_text": "debug"}
			raw, _ := json.Marshal(in)
			agent, _ := Lookup("copilot")
			ev, err := Decode(agent.Adapter, raw, Invocation{Agent: "copilot", Event: host})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, nil)
			st := e.state()
			if len(st.Proposals) != 1 || len(st.Items) != 0 || !strings.Contains(out.String(), "obit approve") {
				t.Fatalf("proposal not delivered: %+v %s", st, out.String())
			}
			var response map[string]any
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response["additionalContext"] == nil || response["hookSpecificOutput"] != nil {
				t.Fatalf("wrong context envelope: %s", out.String())
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "debug.log" {
				t.Fatalf("user file changed: %q %v", data, err)
			}
		})
	}
}

func TestCopilotSessionIdentityAndLifecycle(t *testing.T) {
	t.Parallel()
	id := strings.Repeat("会话", 3000)
	for _, host := range []string{"sessionStart", "agentStop", "sessionEnd"} {
		in := map[string]any{"sessionId": id, "cwd": testenv.NativeFixture("C:/repo"), "source": "startup"}
		raw, _ := json.Marshal(in)
		ev, err := Decode(copilotAdapter{}, raw, Invocation{Agent: "copilot", Event: host})
		if err != nil || ev.Session != core.NormalizeSession("copilot:"+id) {
			t.Fatalf("%+v %v", ev, err)
		}
		delete(in, "sessionId")
		raw, _ = json.Marshal(in)
		if _, err := Decode(copilotAdapter{}, raw, Invocation{Agent: "copilot", Event: host}); !errors.Is(err, ErrSkip) {
			t.Fatalf("missing session: %v", err)
		}
	}
}

func TestCopilotRepliesPreservePermissionAndEndingSilence(t *testing.T) {
	t.Parallel()
	a := copilotAdapter{}
	for _, host := range []string{"preToolUse", "PreToolUse"} {
		data, err := a.Encode(Event{Name: "PreToolUse", HostEvent: host}, Result{Deny: true, Reason: "obit rm a.log", Context: "unrelated"})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		json.Unmarshal(data, &out)
		if host == "PreToolUse" {
			out = out["hookSpecificOutput"].(map[string]any)
		}
		if out["permissionDecision"] != "deny" || out["permissionDecisionReason"] != "obit rm a.log" || strings.Contains(string(data), "additionalContext") {
			t.Fatal(string(data))
		}
		neutral, _ := a.Encode(Event{Name: "PreToolUse", HostEvent: host}, Result{Context: "ignored", Reason: "ignored"})
		if len(neutral) != 0 {
			t.Fatalf("non-deny pre output: %s", neutral)
		}
	}
	for _, event := range []string{"Stop", "SessionEnd"} {
		data, _ := a.Encode(Event{Name: event}, Result{Deny: true, Reason: "no", Context: "continue"})
		if len(data) != 0 {
			t.Fatalf("ending output: %s", data)
		}
	}
	if len(a.Fallback(Event{})) != 0 {
		t.Fatal("fallback grants a decision")
	}
	data, _ := a.Encode(Event{Name: "PostToolUse"}, Result{Context: strings.Repeat("中", 2000)})
	var out struct {
		Context string `json:"additionalContext"`
	}
	json.Unmarshal(data, &out)
	if len(out.Context) > maxContext || !utf8.ValidString(out.Context) {
		t.Fatal("context exceeds byte budget")
	}
}

func TestCopilotNativeDeleteGuardDeniesWithoutTouchingFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, tool := range []string{"bash", "powershell"} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()
			if tool == "powershell" {
				testenv.WindowsOnly(t, "PowerShell Remove-Item integration is Windows-only")
			}
			e := newEnv(t)
			path := e.write("scratch.log")
			command := "rm scratch.log"
			if tool == "powershell" {
				command = "Remove-Item -LiteralPath scratch.log"
			}
			input := map[string]any{"cwd": e.root, "toolName": tool, "toolArgs": map[string]any{"command": command}}
			raw, _ := json.Marshal(input)
			agent, _ := Lookup("copilot")
			ev, err := Decode(agent.Adapter, raw, Invocation{Agent: "copilot", Event: "preToolUse"})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, nil)
			var response map[string]any
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatalf("invalid deny: %s %v", out.String(), err)
			}
			reason, _ := response["permissionDecisionReason"].(string)
			if response["permissionDecision"] != "deny" || !strings.Contains(reason, "scratch.log") || !strings.Contains(reason, "restorable") {
				t.Fatalf("not denied with equivalent command: %s", out.String())
			}
			if len(reason) > maxContext || strings.Contains(out.String(), "modifiedArgs") {
				t.Fatal("unsafe denial", out.String())
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "scratch.log" {
				t.Fatalf("user file changed: %q %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(e.engine.Svc.Store.Home, "store.json")); !os.IsNotExist(err) {
				t.Fatal("pre guard opened ledger", err)
			}
		})
	}
}
