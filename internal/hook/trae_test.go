package hook

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// Fixtures cite the official reference in testdata/trae-sources.json.
func TestTraeProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, name := range []string{"trae", "trae-cn"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			agent, ok := Lookup(name)
			if !ok || agent.Capabilities.WriteObservation.Level != Unsupported || agent.Capabilities.DeleteGuard.Level != Unsupported {
				t.Fatalf("unverified capability: %+v", agent)
			}
			input := cGroupFixture(t, "trae-start")
			ev := cGroupDecode(t, agent, input, "")
			if ev.Name != "SessionStart" || ev.Session != name+":trae-session" || ev.Source != "startup" {
				t.Fatalf("start: %+v", ev)
			}
			cGroupInputErrors(t, agent, input, "", "hook_event_name", "session_id")
			cGroupGuardSkipped(t, agent, cGroupFixture(t, "trae-pre"), "")
			for _, response := range []any{nil, "success", map[string]any{"success": true}, map[string]any{"is_error": true}} {
				post := cGroupFixture(t, "trae-post")
				post["tool_response"] = response
				post["tool_input"] = map[string]any{"file_path": "C:/repo/debug.log"}
				cGroupNoObservation(t, agent, post, "")
			}
			cGroupFinish(t, agent, cGroupFixture(t, "trae-stop"), "", false)
			cGroupResponse(t, agent, true, "hookSpecificOutput", "additionalContext")
			cGroupNeutralFailures(t, agent)
			cGroupExactResponse(t, agent, "SessionStart", Result{Context: "proposal"}, `{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"proposal"}}`)
			cGroupExactResponse(t, agent, "PreToolUse", Result{Deny: true, Reason: "reason"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"reason"}}`)
			for _, event := range []string{"SessionEnd", "PostToolUseFailure"} {
				input["hook_event_name"] = event
				if _, err := Decode(agent.Adapter, cGroupRaw(t, input), Invocation{Agent: name}); !errors.Is(err, ErrSkip) {
					t.Fatalf("unsupported event %s: %v", event, err)
				}
			}
			input["hook_event_name"] = "SessionStart"
			for _, cwd := range []string{"relative", "/home/remote", "ssh://server/repo"} {
				input["cwd"] = cwd
				if _, err := Decode(agent.Adapter, cGroupRaw(t, input), Invocation{Agent: name}); err == nil {
					t.Fatalf("accepted cwd %s", cwd)
				}
			}
		})
	}
}

func TestTraeStartDeliversProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, name := range []string{"trae", "trae-cn"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.wrote(e.write("debug.log"))
			in := cGroupFixture(t, "trae-start")
			in["cwd"] = e.root
			agent, _ := Lookup(name)
			ev := cGroupDecode(t, agent, in, "")
			var out bytes.Buffer
			RunEvent(agent, e.engine, ev, &out, &Runner{Tick: func() {}})
			if !strings.Contains(out.String(), "obit approve") || len(e.state().Items) != 0 {
				t.Fatalf("proposal missing: %s %+v", out.String(), e.state())
			}
		})
	}
}

// Small protocol helpers shared by this group's paired adapter tests.
func cGroupFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	return input
}

func cGroupRaw(t *testing.T, in map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cGroupDecode(t *testing.T, agent Agent, in map[string]any, event string) Event {
	t.Helper()
	raw := cGroupRaw(t, in)
	invocation := Invocation{Agent: agent.Name, Event: event}
	ev, err := Decode(agent.Adapter, raw, invocation)
	if err != nil {
		t.Fatal(err)
	}
	bom, err := Decode(agent.Adapter, append([]byte("\xef\xbb\xbf"), raw...), invocation)
	if err != nil || !reflect.DeepEqual(ev, bom) {
		t.Fatalf("BOM: %+v %v", bom, err)
	}
	return ev
}

func cGroupInputErrors(t *testing.T, agent Agent, input map[string]any, event, eventKey, sessionKey string) {
	t.Helper()
	inv := Invocation{Agent: agent.Name, Event: event}
	for _, raw := range []string{"", "{", "null", "[]", `{"cwd":42}`, `{} {}`} {
		if _, err := Decode(agent.Adapter, []byte(raw), inv); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	saved := input[sessionKey]
	delete(input, sessionKey)
	if _, err := Decode(agent.Adapter, cGroupRaw(t, input), inv); err == nil {
		t.Error("missing session accepted")
	}
	input[sessionKey] = strings.Repeat("s", 4097)
	long := cGroupDecode(t, agent, input, event)
	if !strings.HasPrefix(long.Session, "sha256:") || len(long.Session) != 71 {
		t.Fatalf("session budget: %s", long.Session)
	}
	input[sessionKey] = saved
	if eventKey != "" {
		saved = input[eventKey]
		input[eventKey] = "Unknown"
		if _, err := Decode(agent.Adapter, cGroupRaw(t, input), inv); err == nil {
			t.Error("unknown event accepted")
		}
		input[eventKey] = saved
	}
	inv.Event = "Unknown"
	if _, err := Decode(agent.Adapter, cGroupRaw(t, input), inv); err == nil {
		t.Error("unknown invocation accepted")
	}
}

func cGroupGuardSkipped(t *testing.T, agent Agent, input map[string]any, event string) {
	t.Helper()
	delete(input, "session_id")
	delete(input, "conversationId")
	for _, cwd := range []string{"C:/repo", "/home/remote", "ssh://server/repo"} {
		input["cwd"] = cwd
		input["tool_input"] = map[string]any{"command": "rm scratch.log", "shell": "bash", "local": true}
		if tool, ok := input["toolCall"].(map[string]any); ok {
			tool["args"] = map[string]any{"CommandLine": "rm scratch.log", "Cwd": cwd, "shell": "bash"}
		}
		ev := cGroupDecode(t, agent, input, event)
		if ev.SkipReason == "" || ev.Shell != "" || ev.Command != "rm scratch.log" || ev.Session != "" {
			t.Fatalf("untrusted shell/locality: %+v", ev)
		}
		e := newEnv(t)
		var out bytes.Buffer
		RunEvent(agent, e.engine, ev, &out, nil)
		if out.String() != string(agent.Adapter.Fallback(ev)) {
			t.Fatalf("unexpected decision: %s", out.String())
		}
		if _, err := os.Stat(filepath.Join(e.engine.Svc.Store.Home, "store.json")); !os.IsNotExist(err) {
			t.Fatalf("pre opened ledger: %v", err)
		}
	}
	input["tool_name"] = "code_execution"
	if tool, ok := input["toolCall"].(map[string]any); ok {
		tool["name"] = "code_execution"
	}
	ev := cGroupDecode(t, agent, input, event)
	if ev.Command != "" || ev.SkipReason == "" {
		t.Fatalf("code-mode: %+v", ev)
	}
}

func cGroupNoObservation(t *testing.T, agent Agent, in map[string]any, event string) {
	t.Helper()
	e := newEnv(t)
	in["cwd"] = e.root
	ev, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: agent.Name, Event: event})
	if !errors.Is(err, ErrSkip) || len(ev.Paths) != 0 {
		t.Fatalf("unconfirmed write: %+v %v", ev, err)
	}
	var out bytes.Buffer
	Run(agent.Name, e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, nil)
	if len(e.state().Items) != 0 || len(e.state().Proposals) != 0 {
		t.Fatal("unconfirmed write touched ledger")
	}
}

func cGroupFinish(t *testing.T, agent Agent, input map[string]any, event string, final bool) {
	t.Helper()
	e := newEnv(t)
	input["cwd"] = e.root
	input["workspacePaths"] = []string{e.root}
	ev := cGroupDecode(t, agent, input, event)
	called := 0
	runner := &Runner{Tick: func() {}, Finish: func(root, session string, last bool) {
		called++
		if root != e.root || !strings.HasPrefix(session, agent.Name+":") || last != final {
			t.Errorf("finish: %s %s %v", root, session, last)
		}
	}}
	var out bytes.Buffer
	RunEvent(agent, e.engine, ev, &out, runner)
	if called != 1 || out.String() != string(agent.Adapter.Fallback(ev)) {
		t.Fatalf("finish=%d output=%s", called, out.String())
	}
}

func cGroupResponse(t *testing.T, agent Agent, context bool, outer, field string) {
	t.Helper()
	large := strings.Repeat("中文 <>&\n", 800)
	for _, event := range []string{"Stop", "SessionEnd", "Unknown"} {
		ev := Event{Name: event}
		got, err := agent.Adapter.Encode(ev, Result{Deny: true, Reason: large, Context: large})
		if err != nil || string(got) != string(agent.Adapter.Fallback(ev)) {
			t.Fatalf("ending output: %s %v", got, err)
		}
	}
	for _, event := range []string{"PreToolUse", "SessionStart", "PostToolUse"} {
		ev := Event{Name: event}
		got, err := agent.Adapter.Encode(ev, Result{Deny: true, Reason: large, Context: large})
		if err != nil {
			t.Fatal(err)
		}
		if event == "PreToolUse" || context {
			if len(got) > 0 && !json.Valid(got) {
				t.Fatalf("bad response: %s", got)
			}
		}
		if !utf8.Valid(got) || bytes.Contains(got, []byte(`\u003c`)) {
			t.Fatalf("encoding: %s", got)
		}
		var decoded map[string]any
		if len(got) != 0 && json.Unmarshal(got, &decoded) != nil {
			t.Fatal("response is not JSON")
		}
		if nested, ok := decoded[outer].(map[string]any); ok {
			if text, ok := nested[field].(string); ok && len(text) > maxContext {
				t.Fatalf("context exceeds budget: %d", len(text))
			}
			if reason, ok := nested["permissionDecisionReason"].(string); ok && len(reason) > maxContext {
				t.Fatal("reason exceeds budget")
			}
		}
		if reason, ok := decoded["reason"].(string); ok && len(reason) > maxContext {
			t.Fatal("reason exceeds budget")
		}
		for _, prohibited := range []string{"updatedInput", "overwrite", "auto_approve", "force_continue", "terminationBehavior"} {
			if bytes.Contains(got, []byte(prohibited)) {
				t.Fatalf("unsafe field %s: %s", prohibited, got)
			}
		}
	}
}

func cGroupExactResponse(t *testing.T, agent Agent, event string, result Result, want string) {
	t.Helper()
	got, err := agent.Adapter.Encode(Event{Name: event}, result)
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if json.Unmarshal(got, &actual) != nil || json.Unmarshal([]byte(want), &expected) != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("response: %s want %s", got, want)
	}
}

func cGroupNeutralFailures(t *testing.T, agent Agent) {
	t.Helper()
	for _, raw := range []string{"{", "null", `{"hook_event_name":"Unknown"}`} {
		e := newEnv(t)
		var out bytes.Buffer
		Run(agent.Name, e.engine, strings.NewReader(raw), &out, nil)
		if out.String() != string(agent.Adapter.Fallback(Event{})) {
			t.Fatalf("non-neutral fallback: %s", out.String())
		}
		if _, err := os.Stat(filepath.Join(e.engine.Svc.Store.Home, "store.json")); !os.IsNotExist(err) {
			t.Fatalf("malformed input opened ledger: %v", err)
		}
	}
}
