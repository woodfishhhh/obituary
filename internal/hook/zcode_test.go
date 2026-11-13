package hook

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/agentid"
)

func TestZCodeProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, _ := Lookup(agentid.ZCode)
	for _, name := range []string{"start", "post", "patch", "stop", "pre"} {
		t.Run(name, func(t *testing.T) {
			in := cGroupFixture(t, "zcode-"+name)
			ev := cGroupDecode(t, agent, in, "")
			if !strings.HasPrefix(ev.Session, "zcode:") {
				t.Fatalf("missing namespace: %+v", ev)
			}
			if name == "post" && (ev.Tool != "Write" || len(ev.Paths) != 1 || ev.DeferContext) {
				t.Fatalf("write: %+v", ev)
			}
			if name == "patch" && (!slices.Equal(ev.Paths, []string{testenv.NativeFixture(`C:\work\repo\patch.log`), testenv.NativeFixture(`C:\work\repo\old.log`), testenv.NativeFixture(`C:\work\repo\moved.log`)}) || ev.Tool != "ApplyPatch") {
				t.Fatalf("patch paths: %+v", ev)
			}
			if name == "pre" && (ev.Command != "rm -rf scratch" || ev.SkipReason == "") {
				t.Fatalf("unknown shell must not be inferred from tool name: %+v", ev)
			}
		})
	}
	cGroupExactResponse(t, agent, "PreToolUse", Result{Deny: true, Reason: "obit rm scratch"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"obit rm scratch"}}`)
	cGroupNeutralFailures(t, agent)
	if agent.Capabilities.SessionEnd.Level != Unsupported || agent.Capabilities.DeleteGuard.Level != Limited || agent.Capabilities.SessionStart.ActuallyVerified() {
		t.Fatal("unverified capabilities", agent.Capabilities)
	}
}

func TestZCodeAliasesAndConflicts(t *testing.T) {
	t.Parallel()
	a := zcodeAdapter{}
	for _, only := range []string{"camel", "snake", "both"} {
		in := cGroupFixture(t, "zcode-post")
		for _, pair := range [][2]string{{"hookEventName", "hook_event_name"}, {"sessionId", "session_id"}, {"toolName", "tool_name"}, {"toolInput", "tool_input"}, {"toolResponse", "tool_response"}} {
			if only == "camel" {
				delete(in, pair[1])
			}
			if only == "snake" {
				delete(in, pair[0])
			}
		}
		if ev, err := a.Decode(cGroupRaw(t, in), Invocation{}); err != nil || len(ev.Paths) != 1 {
			t.Fatalf("%s aliases: %+v %v", only, ev, err)
		}
	}
	for _, key := range []string{"hookEventName", "sessionId", "toolName", "toolInput", "toolResponse"} {
		t.Run(key, func(t *testing.T) {
			in := cGroupFixture(t, "zcode-post")
			in[key] = "conflicting"
			if _, err := a.Decode(cGroupRaw(t, in), Invocation{}); err == nil {
				t.Fatal("conflicting fields accepted")
			}
		})
	}
	in := cGroupFixture(t, "zcode-post")
	if _, err := a.Decode(cGroupRaw(t, in), Invocation{Event: "PreToolUse"}); err == nil {
		t.Fatal("CLI event conflict accepted")
	}
}

func TestZCodeRejectsUnprovenWrites(t *testing.T) {
	t.Parallel()
	a := zcodeAdapter{}
	for _, name := range []string{"failed", "untrusted"} {
		if _, err := a.Decode(cGroupRaw(t, cGroupFixture(t, "zcode-"+name)), Invocation{}); err == nil {
			t.Fatal("non-write accepted", name)
		}
	}
	for _, edit := range []func(map[string]any){
		func(in map[string]any) { in["hookEventName"], in["hook_event_name"] = "SessionEnd", "SessionEnd" },
		func(in map[string]any) { in["toolName"], in["tool_name"] = "write", "write" },
		func(in map[string]any) { delete(in, "sessionId"); delete(in, "session_id") },
		func(in map[string]any) { delete(in, "toolResponse"); delete(in, "tool_response") },
		func(in map[string]any) { in["toolResponse"], in["tool_response"] = nil, nil },
		func(in map[string]any) { in["error"] = "failed" },
		func(in map[string]any) { in["cwd"] = testenv.ForeignFixture("/home/remote/project") },
		func(in map[string]any) { in["cwd"] = "ssh://server/project" },
		func(in map[string]any) {
			in["toolInput"], in["tool_input"] = map[string]any{"file_path": "relative.log"}, map[string]any{"file_path": "relative.log"}
		},
	} {
		in := cGroupFixture(t, "zcode-post")
		edit(in)
		if ev, err := a.Decode(cGroupRaw(t, in), Invocation{}); err == nil {
			t.Fatalf("unproven write accepted: %+v", ev)
		}
	}
	for _, key := range []string{"command", "patch", "input"} {
		in := cGroupFixture(t, "zcode-patch")
		in["toolInput"] = map[string]any{key: "*** Begin Patch\n*** Add File: wrong.log\n+x\n*** End Patch"}
		if _, err := a.Decode(cGroupRaw(t, in), Invocation{}); err == nil {
			t.Fatal("non-ZCode patch field accepted", key)
		}
	}
}

func TestZCodeObservationAndSessionIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	path := e.write("debug.log")
	run := func(fixture, session string) string {
		in := cGroupFixture(t, "zcode-"+fixture)
		in["cwd"] = e.root
		in["sessionId"], in["session_id"] = session, session
		if fixture == "post" || fixture == "failed" {
			in["toolInput"], in["tool_input"] = map[string]any{"file_path": path}, map[string]any{"file_path": path}
		}
		var out bytes.Buffer
		Run(agentid.ZCode, e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, &Runner{Tick: func() {}})
		return out.String()
	}
	if out := run("failed", "main"); out != "" || len(e.state().Proposals) != 0 {
		t.Fatal("failed tool observed", out)
	}
	if out := run("post", "main"); !strings.Contains(out, "additionalContext") {
		t.Fatal("missing post notice", out)
	}
	st := e.state()
	if len(st.Proposals) != 1 || st.Proposals[0].Session != "zcode:main" || !slices.Equal(st.Proposals[0].Notified, []string{"zcode:main"}) {
		t.Fatalf("main notice: %+v", st.Proposals)
	}
	if out := run("start", "subagent_worker"); !strings.Contains(out, "additionalContext") {
		t.Fatal("child session did not receive its own notice", out)
	}
	if !slices.Contains(e.state().Proposals[0].Notified, "zcode:subagent_worker") {
		t.Fatal("child acknowledgement missing")
	}
	var finished string
	in := cGroupFixture(t, "zcode-stop")
	in["cwd"], in["sessionId"] = e.root, "subagent_worker"
	var out bytes.Buffer
	Run(agentid.ZCode, e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, &Runner{Finish: func(root, session string, final bool) {
		if root != e.root || final {
			t.Errorf("unexpected finish: %s %v", root, final)
		}
		finished = session
	}})
	if out.Len() != 0 || finished != "zcode:subagent_worker" {
		t.Fatalf("Stop: %s %q", out.String(), finished)
	}
}

func TestZCodeContextBudgetAndSilence(t *testing.T) {
	t.Parallel()
	a := zcodeAdapter{}
	for _, event := range []Event{{Name: "SessionStart"}, {Name: "PostToolUse"}, {Name: "Context", HostEvent: "UserPromptSubmit"}} {
		raw, err := a.Encode(event, Result{Context: strings.Repeat("文", 2000), Deny: true, Reason: "unrelated"})
		var out claudeOutput
		if err != nil || json.Unmarshal(raw, &out) != nil || len(out.HookSpecificOutput.AdditionalContext) > maxContext || out.HookSpecificOutput.PermissionDecision != "" {
			t.Fatalf("invalid context: %s %v", raw, err)
		}
	}
	for _, event := range []Event{{Name: "Stop"}, {Name: "SessionEnd"}, {Name: "PostToolUse", DeferContext: true}} {
		if raw, err := a.Encode(event, Result{Deny: true, Reason: "deny", Context: "notice"}); err != nil || len(raw) != 0 {
			t.Fatalf("event must stay silent: %s %v", raw, err)
		}
	}
	// Tool output strings are never used as candidate paths.
	in := cGroupFixture(t, "zcode-patch")
	in["toolInput"] = map[string]any{"patch_text": "*** Begin Patch\n*** Add File: safe.log\n+*** Add File: fake.log\n*** End Patch"}
	ev, err := a.Decode(cGroupRaw(t, in), Invocation{})
	if err != nil || len(ev.Paths) != 1 || filepath.Base(ev.Paths[0]) != "safe.log" {
		t.Fatalf("patch headers: %+v %v", ev, err)
	}
}
