package hook

import "fmt"

// The plugin translates before_tool_call/after_tool_call and lifecycle events
// into bridge v1. In particular, exec with toolKind=code_mode_exec or
// toolInputKind=javascript must carry skip_reason; remote/sandbox tools must
// leave local=false. See docs.openclaw.ai/plugins/hooks/tool-policy and
// prompt-and-session (2026-11-09) for the block/blockReason response contract.
type openclawAdapter struct{ BridgeAdapter }

func (a openclawAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	ev, err := a.BridgeAdapter.Decode(raw, invocation)
	if err != nil {
		return ev, err
	}
	if ev.Name == "Context" && ev.HostEvent != "before_prompt_build" {
		return Event{}, fmt.Errorf("OpenClaw context requires before_prompt_build")
	}
	// These callbacks ignore returned text; Context is reserved for the
	// granted before_prompt_build channel supplied by the plugin.
	if ev.Name == "PostToolUse" || ev.Name == "SessionStart" {
		ev.DeferContext = true
	}
	return ev, nil
}

func init() {
	Register(Agent{Name: "openclaw", Adapter: openclawAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "before_tool_call requires confirmed local shell exec; remote, sandbox and Code Mode are skipped; real host validation pending"},
		WriteObservation: Support{Level: Limited, Detail: "after_tool_call observes successful local writes and leaves notices pending; plugin round trip tested"},
		SessionStart:     Support{Level: Limited, Detail: "session_start takes a snapshot without acknowledging notices"},
		Stop:             Support{Level: Limited, Detail: "agent_end maps to silent Stop"},
		SessionEnd:       Support{Level: Limited, Detail: "session_end maps to silent SessionEnd; shutdown shares a 2-second drain budget and requires background handoff"},
		Context:          Support{Level: Limited, Detail: "before_prompt_build requires allowConversationAccess=true and allowPromptModification not false; plugin round trip tested, real host validation pending"},
		MCP:              Support{Level: Limited, Detail: "Official mcp.servers stdio entries; Gateway ownership and native/WSL/remote target require phase-two installation checks"},
		Skill:            Support{Level: Unsupported, Detail: "Skill installation and ownership are not implemented for this agent"},
	}})
}
