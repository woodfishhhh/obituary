package hook

// Pi and OMP share bridge v1 after their plugin extracts tool_call.input and
// successful tool_result.input. The plugin obtains the stable session from
// ctx.sessionManager and verifies the actual shell/local execution backend.
// Sources: badlogic/pi-mono packages/coding-agent/docs/extensions.md at
// 7d073328e44337d3292a7db9b0f0308f679c5e97.
type piAdapter struct{ BridgeAdapter }

func init() {
	Register(Agent{Name: "pi", Adapter: piAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "Bridge v1 only; tool_call uses block/reason after confirmed local shell evidence; plugin wiring pending"},
		WriteObservation: Support{Level: Limited, Detail: "Successful tool_result write/edit input.path only; plugin extraction and model-visible result delivery pending"},
		SessionStart:     Support{Level: Limited, Detail: "session_start needs a stable sessionManager id; snapshot taken even when passive context delivery is unavailable"},
		Stop:             Support{Level: Limited, Detail: "agent_settled is final idle; older agent_end is only a run boundary; both require silent Stop"},
		SessionEnd:       Support{Level: Limited, Detail: "session_shutdown maps to silent SessionEnd; plugin background handoff pending"},
		Context:          Support{Level: Limited, Detail: "tool_result content or sendMessage without triggerTurn; callback must explicitly confirm delivery capability"},
		MCP:              Support{Level: Limited, Detail: "Current Pi supports native mcp.json; installation belongs to phase two"},
		Skill:            Support{Level: Unsupported, Detail: "Skill installation and ownership are not implemented for this agent"},
	}})
}
