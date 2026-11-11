package hook

import "fmt"

// Kilo's current plugin interface is separate from legacy Roo-based Kilo.
// See kilo.ai/docs/automate/extending/plugins and packages/plugin/src/index.ts.
type kilocodeAdapter struct{ BridgeAdapter }

func (a kilocodeAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	if invocation.Agent != "kilocode" {
		return Event{}, fmt.Errorf("Kilo adapter requires the kilocode invocation") //nolint:staticcheck // ST1005: Kilo is the host's proper name in hook diagnostics.
	}
	ev, err := a.BridgeAdapter.Decode(raw, invocation)
	if err != nil {
		return Event{}, err
	}
	var name string
	switch ev.HostEvent {
	case "tool.execute.before":
		name = "PreToolUse"
	case "tool.execute.after":
		name = "PostToolUse"
		// Keep proposals unacknowledged until the plugin can deliver context.
		ev.DeferContext = true
	case "session.created":
		name = "SessionStart"
		ev.DeferContext = true
	case "session.idle":
		name = "Stop"
	case "session.deleted":
		name = "SessionEnd"
	case "experimental.chat.system.transform":
		name = "Context" // documented prompt-array channel; plugin must attest delivery
	default:
		return Event{}, ErrSkip
	}
	if ev.Name != name {
		return Event{}, fmt.Errorf("Kilo %s requires %s", ev.HostEvent, name) //nolint:staticcheck // ST1005: Kilo is the host's proper name in hook diagnostics.
	}
	if name == "PreToolUse" && ev.Tool != "bash" {
		ev.SkipReason = "Kilo: unsupported shell tool"
	}
	if name == "PostToolUse" && ev.Tool != "write" && ev.Tool != "edit" && ev.Tool != "apply_patch" {
		return Event{}, ErrSkip
	}
	return ev, nil
}

func init() {
	Register(Agent{Name: "kilocode", Adapter: kilocodeAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "current PluginModule tool-error veto; OBIT_KILOCODE_LOCAL_SHELL=bash or powershell requires user-verified local executor"},
		WriteObservation: Support{Level: Limited, Detail: "tool.execute.after observes successful official file-tool args without acknowledging notices"},
		SessionStart:     Support{Level: Limited, Detail: "session.created takes a snapshot without acknowledging notices"},
		Stop:             Support{Level: Limited, Detail: "session.idle maps to silent Stop"},
		SessionEnd:       Support{Level: Limited, Detail: "session.deleted handles explicit persistent-session deletion only; ordinary cleanup uses Stop"},
		Context:          Support{Level: Limited, Detail: "experimental.chat.system.transform Context bridge with context_delivery=true"},
		MCP:              Support{Level: Limited, Detail: "current kilo.json[c] local mcp entries; ambiguous layered files require manual consolidation; legacy .kilocode MCP is separate"},
		Skill:            Support{Level: Unsupported, Detail: "Skill installation and ownership are not implemented for this agent"},
	}})
}
