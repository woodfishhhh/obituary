package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAntigravityProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, _ := Lookup("antigravity")
	in := cGroupFixture(t, "antigravity-start")
	ev := cGroupDecode(t, agent, in, "SessionStart")
	if ev.Session != "antigravity:agy-session" || ev.Cwd != "C:/repo" {
		t.Fatalf("start: %+v", ev)
	}
	cGroupInputErrors(t, agent, in, "SessionStart", "", "conversationId")
	cGroupGuardSkipped(t, agent, cGroupFixture(t, "antigravity-pre"), "PreToolUse")
	cGroupFinish(t, agent, cGroupFixture(t, "antigravity-stop"), "Stop", false)
	cGroupResponse(t, agent, true, "", "")
	cGroupNeutralFailures(t, agent)
	cGroupExactResponse(t, agent, "PreToolUse", Result{Deny: true, Reason: "reason"}, `{"decision":"deny","reason":"reason"}`)
	cGroupExactResponse(t, agent, "SessionStart", Result{Context: "proposal"}, `{"injectSteps":[{"ephemeralMessage":"proposal"}]}`)
	for _, event := range []string{"", "SessionEnd", "PostInvocation"} {
		if _, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: agent.Name, Event: event}); err == nil {
			t.Fatalf("unsupported event %q", event)
		}
	}
	data, err := agent.Adapter.Encode(Event{Name: "SessionStart"}, Result{Context: strings.Repeat("中", 3000)})
	var response struct {
		Steps []struct {
			Context string `json:"ephemeralMessage"`
		} `json:"injectSteps"`
	}
	if err != nil || json.Unmarshal(data, &response) != nil || len(response.Steps) != 1 || len(response.Steps[0].Context) > maxContext {
		t.Fatalf("inject budget: %s %v", data, err)
	}
	if string(agent.Adapter.Fallback(Event{Name: "PreToolUse"})) != "{\"decision\":\"allow\"}\n" || string(agent.Adapter.Fallback(Event{})) != "{}\n" {
		t.Fatal("invalid fallback")
	}
}

func TestAntigravityRoutesWriteToMatchingWorkspace(t *testing.T) {
	t.Parallel()
	agent, _ := Lookup("antigravity")
	for _, tool := range []string{"write_to_file", "replace_file_content", "multi_replace_file_content"} {
		in := cGroupFixture(t, "antigravity-post")
		in["toolCall"].(map[string]any)["name"] = tool
		ev := cGroupDecode(t, agent, in, "PostToolUse")
		if ev.Cwd != "C:/second" || len(ev.Paths) != 1 || ev.Paths[0] != "C:/second/debug.log" || !ev.DeferContext {
			t.Fatalf("routing: %+v", ev)
		}
	}
	for _, change := range []func(map[string]any){
		func(in map[string]any) { in["error"] = "write failed" },
		func(in map[string]any) { in["workspacePaths"] = []string{"C:/elsewhere"} },
		func(in map[string]any) { in["toolCall"].(map[string]any)["name"] = "code_execution" },
		func(in map[string]any) { in["toolCall"].(map[string]any)["name"] = "mcp_remote.write_to_file" },
		func(in map[string]any) {
			in["toolCall"].(map[string]any)["args"] = map[string]any{"TargetFile": "/home/remote/debug.log"}
		},
		func(in map[string]any) {
			in["toolCall"].(map[string]any)["args"] = map[string]any{"TargetFile": "debug.log"}
		},
	} {
		in := cGroupFixture(t, "antigravity-post")
		change(in)
		if ev, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: agent.Name, Event: "PostToolUse"}); err == nil || len(ev.Paths) != 0 {
			t.Fatalf("unsafe write: %+v %v", ev, err)
		}
	}
	for _, event := range []string{"SessionStart", "Stop"} {
		in := cGroupFixture(t, "antigravity-post")
		if _, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: agent.Name, Event: event}); err == nil {
			t.Fatal("multi-root lifecycle chose first workspace")
		}
	}
}

func TestAntigravityDeferredWritePreservesLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	in := cGroupFixture(t, "antigravity-post")
	in["workspacePaths"] = []string{"C:/unrelated", e.root}
	in["toolCall"].(map[string]any)["args"].(map[string]any)["TargetFile"] = path
	agent, _ := Lookup("antigravity")
	ev := cGroupDecode(t, agent, in, "PostToolUse")
	var out bytes.Buffer
	RunEvent(agent, e.engine, ev, &out, nil)
	if out.String() != "{}\n" {
		t.Fatal(out.String())
	}
	if st := e.state(); len(st.Proposals) != 1 || len(st.Proposals[0].Notified) != 0 {
		t.Fatalf("deferred observation lost or acknowledged: %+v", st.Proposals)
	}
	in["workspacePaths"] = []string{e.root}
	ev = cGroupDecode(t, agent, in, "PreInvocation")
	RunEvent(agent, e.engine, ev, &out, nil)
	if !strings.Contains(out.String(), "obit approve") || len(e.state().Proposals[0].Notified) != 1 {
		t.Fatalf("PreInvocation did not deliver pending notice: %s", out.String())
	}
}
