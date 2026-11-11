package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestKimiBetaProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, _ := Lookup("kimi")
	in := cGroupFixture(t, "kimi-start")
	ev := cGroupDecode(t, agent, in, "")
	if ev.Session != "kimi:kimi-session" || !ev.DeferContext || ev.Source != "startup" {
		t.Fatalf("start: %+v", ev)
	}
	cGroupInputErrors(t, agent, in, "", "hook_event_name", "session_id")
	cGroupGuardSkipped(t, agent, cGroupFixture(t, "kimi-pre"), "")
	cGroupFinish(t, agent, cGroupFixture(t, "kimi-stop"), "", false)
	cGroupFinish(t, agent, cGroupFixture(t, "kimi-end"), "", true)
	cGroupResponse(t, agent, false, "hookSpecificOutput", "additionalContext")
	cGroupNeutralFailures(t, agent)
	cGroupExactResponse(t, agent, "PreToolUse", Result{Deny: true, Reason: "reason"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"reason"}}`)
	if agent.Capabilities.WriteObservation.Level != Unsupported || agent.Capabilities.Context.Level != Unsupported {
		t.Fatalf("unverified capabilities: %+v", agent.Capabilities)
	}
}

func TestKimiBetaDoesNotInferWriteSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, _ := Lookup("kimi")
	for _, result := range []string{"", "success", "message='File written successfully.'", "message='Permission denied' brief='Denied'"} {
		in := cGroupFixture(t, "kimi-post")
		in["tool_output"] = result
		cGroupNoObservation(t, agent, in, "")
	}
	in := cGroupFixture(t, "kimi-post")
	in["hook_event_name"] = "PostToolUseFailure"
	cGroupNoObservation(t, agent, in, "")
}

func TestKimiBetaDefersUndeliverableStart(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	e := newEnv(t)
	in := cGroupFixture(t, "kimi-start")
	in["cwd"] = e.root
	var out bytes.Buffer
	Run("kimi", e.engine, bytes.NewReader(cGroupRaw(t, in)), &out, nil)
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
	if _, err := os.Stat(filepath.Join(e.engine.Svc.Store.Home, "store.json")); !os.IsNotExist(err) {
		t.Fatalf("unverified context delivery touched ledger: %v", err)
	}
	for _, event := range []string{"SessionStart", "PostToolUse"} {
		data, err := (kimiAdapter{}).Encode(Event{Name: event}, Result{Context: "not delivered"})
		if err != nil || len(data) != 0 {
			t.Fatalf("unexpected context %s %v", data, err)
		}
	}
}
