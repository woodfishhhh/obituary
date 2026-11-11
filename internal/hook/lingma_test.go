package hook

import (
	"errors"
	"testing"
)

func TestLingmaProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	agent, ok := Lookup("lingma")
	if !ok || agent.Capabilities.SessionStart.Level != Unsupported || agent.Capabilities.SessionEnd.Level != Unsupported {
		t.Fatalf("invented lifecycle: %+v", agent)
	}
	cGroupGuardSkipped(t, agent, cGroupFixture(t, "lingma-pre"), "")
	in := cGroupFixture(t, "lingma-post")
	ev := cGroupDecode(t, agent, in, "")
	if ev.Session != "lingma:lingma-session" || len(ev.Paths) != 1 || !ev.DeferContext {
		t.Fatalf("post: %+v", ev)
	}
	cGroupInputErrors(t, agent, in, "", "hook_event_name", "session_id")
	for _, event := range []string{"SessionStart", "SessionEnd", "PostToolUseFailure"} {
		in["hook_event_name"] = event
		if _, err := Decode(agent.Adapter, cGroupRaw(t, in), Invocation{Agent: "lingma"}); !errors.Is(err, ErrSkip) {
			t.Fatalf("unsupported event %s: %v", event, err)
		}
	}
	cGroupFinish(t, agent, cGroupFixture(t, "lingma-stop"), "", false)
	cGroupNeutralFailures(t, agent)
}
