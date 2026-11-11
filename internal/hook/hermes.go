package hook

import "fmt"

// The Python plugin converts official callback arguments to BridgeRequest.
// See hermes-agent.nousresearch.com/docs/developer-guide/plugins. The plugin
// owns transport; shared hook handling retains the safety decisions.
type hermesAdapter struct{ BridgeAdapter }

func (a hermesAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	if invocation.Agent != "hermes" {
		return Event{}, fmt.Errorf("Hermes adapter requires the hermes invocation") //nolint:staticcheck // ST1005: Hermes is the host's proper name in hook diagnostics.
	}
	ev, err := a.BridgeAdapter.Decode(raw, invocation)
	if err != nil {
		return Event{}, err
	}
	var name string
	switch ev.HostEvent {
	case "pre_tool_call":
		name = "PreToolUse"
	case "post_tool_call":
		name = "PostToolUse"
		ev.DeferContext = true // Hermes ignores post_tool_call return values.
	case "on_session_start":
		name = "SessionStart"
		ev.DeferContext = true
	case "on_session_end":
		// This callback fires after every run_conversation, including turns.
		name = "Stop"
	case "pre_llm_call":
		name = "Context"
	default:
		return Event{}, ErrSkip
	}
	if ev.Name != name {
		return Event{}, fmt.Errorf("Hermes %s requires %s", ev.HostEvent, name) //nolint:staticcheck // ST1005: Hermes is the host's proper name in hook diagnostics.
	}
	if name == "PreToolUse" && ev.Tool != "terminal" {
		ev.SkipReason = "Hermes: unsupported shell tool"
	}
	if name == "PostToolUse" && ev.Tool != "write_file" && ev.Tool != "patch" {
		return Event{}, ErrSkip
	}
	return ev, nil
}

func init() {
	Register(Agent{Name: "hermes", Adapter: hermesAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "global Python plugin; cached LocalEnvironment proves local Bash; first call without backend evidence is skipped; real host validation pending"},
		WriteObservation: Support{Level: Limited, Detail: "post_tool_call observes successful local writes without acknowledging notices; pre_llm_call delivers them"},
		SessionStart:     Support{Level: Limited, Detail: "on_session_start takes a snapshot without acknowledging notices"},
		Stop:             Support{Level: Limited, Detail: "on_session_end maps to silent Stop after each run_conversation; requires local backend evidence"},
		SessionEnd:       Support{Level: Unsupported, Detail: "on_session_end is a turn callback; final teardown callback needs separate version validation"},
		Context:          Support{Level: Limited, Detail: "pre_llm_call Context bridge delivers pending notices with context_delivery=true; real host validation pending"},
		MCP:              Support{Level: Limited, Detail: "global config.yaml mcp_servers; project scope requires global registration"},
		Skill:            Support{Level: Unsupported, Detail: "managed Skill installation has not been verified"},
	}})
}
