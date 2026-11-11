package hook

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

func cursorFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var in map[string]any
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func TestCursorDocumentedEvents(t *testing.T) {
	t.Parallel()
	a, ok := Lookup("cursor")
	if !ok {
		t.Fatal("adapter missing")
	}
	for _, tc := range []struct {
		file, event string
		deferred    bool
	}{{"cursor-pre", "PreToolUse", false}, {"cursor-edit", "PostToolUse", true}, {"cursor-start", "SessionStart", true}} {
		t.Run(tc.file, func(t *testing.T) {
			raw, _ := json.Marshal(cursorFixture(t, tc.file))
			for _, prefix := range []string{"", "\xef\xbb\xbf"} {
				ev, err := Decode(a.Adapter, append([]byte(prefix), raw...), Invocation{Agent: "cursor"})
				if err != nil || ev.Name != tc.event || ev.Session != "cursor:cursor-1" || ev.DeferContext != tc.deferred {
					t.Fatalf("%+v %v", ev, err)
				}
				if ev.Name == "PreToolUse" && (ev.SkipReason == "" || ev.Shell != "") {
					t.Fatal("ambiguous Shell accepted")
				}
			}
		})
	}
}

func TestCursorRejectsUnsupportedAndMissingEvidence(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"{", "[]", `{"workspace_roots":7}`, `{"tool_input":false}`} {
		if _, err := Decode(cursorAdapter{}, []byte(raw), Invocation{Agent: "cursor"}); err == nil {
			t.Fatalf("malformed accepted: %s", raw)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknown", func(m map[string]any) { m["hook_event_name"] = "unknown" }},
		{"failed", func(m map[string]any) { m["hook_event_name"] = "postToolUseFailure"; m["failure_type"] = "error" }},
		{"missing-session", func(m map[string]any) { delete(m, "conversation_id"); m["session_id"] = "not-the-conversation" }},
		{"background", func(m map[string]any) { m["is_background_agent"] = true }},
		{"remote", func(m map[string]any) { m["workspace_roots"] = []string{"vscode-remote://ssh-remote+server/repo"} }},
		{"ambiguous-roots", func(m map[string]any) { m["workspace_roots"] = []string{"C:/one", "C:/two"} }},
		{"relative-file", func(m map[string]any) { m["file_path"] = "debug.log" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := cursorFixture(t, "cursor-edit")
			tc.change(in)
			raw, _ := json.Marshal(in)
			if _, err := Decode(cursorAdapter{}, raw, Invocation{Agent: "cursor"}); !errors.Is(err, ErrSkip) {
				t.Fatalf("accepted: %v", err)
			}
		})
	}
	for _, tool := range []string{"Shell", "MCP:remote_exec", "code"} {
		in := cursorFixture(t, "cursor-pre")
		in["tool_name"] = tool
		delete(in, "conversation_id")
		in["tool_input"] = map[string]any{"command": "rm a.log", "shell": "bash"}
		raw, _ := json.Marshal(in)
		ev, err := Decode(cursorAdapter{}, raw, Invocation{Agent: "cursor"})
		if err != nil || ev.SkipReason == "" {
			t.Fatalf("unverified shell: %+v %v", ev, err)
		}
	}
	raw, _ := json.Marshal(cursorFixture(t, "cursor-start"))
	if _, err := Decode(cursorAdapter{}, raw, Invocation{Agent: "cursor", Event: "sessionEnd"}); err == nil {
		t.Fatal("conflicting event accepted")
	}
}

func TestCursorDeferredWritesLeaveProposalsUnacknowledged(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	in := cursorFixture(t, "cursor-edit")
	in["workspace_roots"] = []string{e.root}
	in["file_path"] = path
	raw, _ := json.Marshal(in)
	agent, _ := Lookup("cursor")
	ev, err := Decode(agent.Adapter, raw, Invocation{Agent: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	RunEvent(agent, e.engine, ev, &out, nil)
	if st := e.state(); len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 || len(st.Items) != 0 || out.Len() != 0 {
		t.Fatal("deferred observation lost or acknowledged")
	}
	in["hook_event_name"] = "postToolUse"
	in["tool_name"] = "Write"
	in["tool_input"] = map[string]any{"file_path": e.write("unverified.log")}
	raw, _ = json.Marshal(in)
	ev, err = Decode(agent.Adapter, raw, Invocation{Agent: "cursor"})
	if err != nil || ev.Name != "Context" || len(ev.Paths) != 0 {
		t.Fatalf("notification callback inferred a write: %+v %v", ev, err)
	}
	RunEvent(agent, e.engine, ev, &out, nil)
	if !strings.Contains(out.String(), "debug.log") || len(e.state().Proposals[0].Candidates) != 1 || len(e.state().Proposals[0].Notified) != 1 {
		t.Fatalf("pending notice not delivered: %s", out.String())
	}
}

func TestCursorLifecycleReplies(t *testing.T) {
	t.Parallel()
	a := cursorAdapter{}
	id := strings.Repeat("x", 5000)
	for _, host := range []string{"sessionStart", "postToolUse", "stop", "sessionEnd"} {
		in := cursorFixture(t, "cursor-start")
		in["hook_event_name"] = host
		in["conversation_id"] = id
		raw, _ := json.Marshal(in)
		ev, err := Decode(a, raw, Invocation{Agent: "cursor"})
		if err != nil || ev.Session != core.NormalizeSession("cursor:"+id) {
			t.Fatalf("%+v %v", ev, err)
		}
		data, _ := a.Encode(ev, Result{Deny: true, Reason: "no", Context: strings.Repeat("中", 2000)})
		if host != "postToolUse" {
			if len(data) != 0 {
				t.Fatalf("ending continued: %s", data)
			}
			continue
		}
		var out struct {
			Context string `json:"additional_context"`
		}
		json.Unmarshal(data, &out)
		if len(out.Context) > maxContext || out.Context == "" || !utf8.ValidString(out.Context) {
			t.Fatal("bad context budget")
		}
	}
	if len(a.Fallback(Event{})) != 0 {
		t.Fatal("fallback must use documented no-output path")
	}
	data, _ := a.Encode(Event{Name: "PreToolUse"}, Result{Deny: true, Reason: "obit rm a.log"})
	if !strings.Contains(string(data), `"permission":"deny"`) || strings.Contains(string(data), "updated_input") {
		t.Fatal(string(data))
	}
	data, _ = a.Encode(Event{Name: "PostToolUse", DeferContext: true}, Result{Context: "not delivered"})
	if len(data) != 0 {
		t.Fatal(string(data))
	}
}
