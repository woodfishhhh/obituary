package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
)

func geminiFixture(t *testing.T, name string) map[string]any {
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

func TestGeminiDocumentedPayloads(t *testing.T) {
	t.Parallel()
	a, ok := Lookup("gemini")
	if !ok {
		t.Fatal("adapter missing")
	}
	for _, tc := range []struct {
		file, event string
		paths       int
	}{{"gemini-pre", "PreToolUse", 0}, {"gemini-write", "PostToolUse", 1}} {
		t.Run(tc.file, func(t *testing.T) {
			raw, _ := json.Marshal(geminiFixture(t, tc.file))
			for _, prefix := range []string{"", "\xef\xbb\xbf"} {
				ev, err := Decode(a.Adapter, append([]byte(prefix), raw...), Invocation{Agent: "gemini"})
				if err != nil || ev.Name != tc.event || ev.Session != "gemini:gemini-1" || len(ev.Paths) != tc.paths {
					t.Fatalf("%+v %v", ev, err)
				}
				if ev.Name == "PreToolUse" && (ev.SkipReason == "" || ev.Shell != "") {
					t.Fatal("shell guessed")
				}
			}
		})
	}
	raw, _ := json.Marshal(geminiFixture(t, "gemini-write-failed"))
	if _, err := Decode(a.Adapter, raw, Invocation{Agent: "gemini"}); !errors.Is(err, ErrSkip) {
		t.Fatal("failed write accepted", err)
	}
}

func TestGeminiIgnoresUnverifiedWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknown", func(m map[string]any) { m["hook_event_name"] = "unknown" }},
		{"missing-session", func(m map[string]any) { delete(m, "session_id") }},
		{"missing-response", func(m map[string]any) { delete(m, "tool_response") }},
		{"missing-content", func(m map[string]any) { m["tool_response"] = map[string]any{} }},
		{"null-content", func(m map[string]any) { m["tool_response"] = map[string]any{"llmContent": nil} }},
		{"failed-response", func(m map[string]any) {
			m["tool_response"] = map[string]any{"llmContent": "pretend success", "error": map[string]any{"message": "write denied"}}
		}},
		{"remote", func(m map[string]any) { m["cwd"] = testenv.ForeignFixture("/workspace/remote") }},
		{"remote-file", func(m map[string]any) {
			m["tool_input"] = map[string]any{"file_path": testenv.ForeignFixture("/workspace/remote/file.log")}
		}},
		{"shell-result-path", func(m map[string]any) { m["tool_name"] = "run_shell_command" }},
		{"code-mode", func(m map[string]any) { m["tool_name"] = "code" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := geminiFixture(t, "gemini-write")
			tc.change(in)
			raw, _ := json.Marshal(in)
			if ev, err := Decode(geminiAdapter{}, raw, Invocation{Agent: "gemini"}); !errors.Is(err, ErrSkip) || len(ev.Paths) != 0 {
				t.Fatalf("unsafe write accepted: %+v %v", ev, err)
			}
		})
	}
	for _, raw := range []string{"{", "[]", `{"tool_input":4}`, `{"tool_response":true}`} {
		if _, err := Decode(geminiAdapter{}, []byte(raw), Invocation{Agent: "gemini"}); err == nil {
			t.Fatal("malformed input accepted", raw)
		}
	}
	in := geminiFixture(t, "gemini-pre")
	delete(in, "session_id")
	in["tool_input"] = map[string]any{"command": "rm a.log", "shell": "bash"}
	raw, _ := json.Marshal(in)
	ev, err := Decode(geminiAdapter{}, raw, Invocation{Agent: "gemini"})
	if err != nil || ev.SkipReason == "" {
		t.Fatalf("unverified shell accepted: %+v %v", ev, err)
	}
	if _, err := Decode(geminiAdapter{}, raw, Invocation{Agent: "gemini", Event: "AfterTool"}); err == nil {
		t.Fatal("event conflict accepted")
	}
}

func TestGeminiSuccessfulWriteObservesRealProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, tool := range []string{"write_file", "replace"} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			path := e.write("debug.log")
			in := geminiFixture(t, "gemini-write")
			in["cwd"] = e.root
			in["tool_name"] = tool
			in["tool_input"] = map[string]any{"file_path": path}
			raw, _ := json.Marshal(in)
			var out bytes.Buffer
			Run("gemini", e.engine, bytes.NewReader(raw), &out, nil)
			st := e.state()
			if len(st.Proposals) != 1 || len(st.Items) != 0 || !strings.Contains(out.String(), "obit approve") {
				t.Fatalf("proposal not delivered: %+v %s", st, out.String())
			}
			var response claudeOutput
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.HookSpecificOutput.HookEventName != "AfterTool" || response.HookSpecificOutput.AdditionalContext == "" {
				t.Fatal("wrong output event", out.String())
			}
		})
	}
}

func TestGeminiFailedWriteDoesNotPropose(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	in := geminiFixture(t, "gemini-write-failed")
	in["cwd"] = e.root
	in["tool_input"] = map[string]any{"file_path": path}
	raw, _ := json.Marshal(in)
	var out bytes.Buffer
	Run("gemini", e.engine, bytes.NewReader(raw), &out, nil)
	if len(e.state().Proposals) != 0 || len(e.state().Items) != 0 || out.String() != "{}\n" {
		t.Fatalf("failure observed: %s", out.String())
	}
}

func TestGeminiLifecycleAndReplies(t *testing.T) {
	t.Parallel()
	a := geminiAdapter{}
	id := strings.Repeat("会", 5000)
	for _, host := range []string{"SessionStart", "AfterAgent", "SessionEnd"} {
		in := map[string]any{"hook_event_name": host, "session_id": id, "cwd": testenv.NativeFixture("C:/repo")}
		raw, _ := json.Marshal(in)
		ev, err := Decode(a, raw, Invocation{Agent: "gemini"})
		if err != nil || ev.Session != core.NormalizeSession("gemini:"+id) {
			t.Fatalf("%+v %v", ev, err)
		}
		data, _ := a.Encode(ev, Result{Deny: true, Reason: "no", Context: strings.Repeat("中", 2000)})
		if host != "SessionStart" {
			if string(data) != "{}\n" {
				t.Fatalf("ending continued: %s", data)
			}
		} else {
			var out claudeOutput
			json.Unmarshal(data, &out)
			text := out.HookSpecificOutput.AdditionalContext
			if len(text) > maxContext || text == "" || !utf8.ValidString(text) {
				t.Fatal("bad context budget")
			}
		}
		delete(in, "session_id")
		raw, _ = json.Marshal(in)
		if _, err := Decode(a, raw, Invocation{Agent: "gemini"}); !errors.Is(err, ErrSkip) {
			t.Fatal("missing session accepted")
		}
	}
	if string(a.Fallback(Event{})) != "{}\n" {
		t.Fatal("invalid fallback")
	}
	data, _ := a.Encode(Event{Name: "PreToolUse"}, Result{Deny: true, Reason: "obit rm a.log", Context: "ignored"})
	if string(data) != "{\"decision\":\"deny\",\"reason\":\"obit rm a.log\"}\n" {
		t.Fatalf("deny envelope: %s", data)
	}
	data, _ = a.Encode(Event{Name: "PreToolUse"}, Result{Context: "ignored"})
	if string(data) != "{}\n" {
		t.Fatal("neutral grants permission")
	}
}
