package hook

// OMP uses Pi's decoder while keeping a separate session namespace and coverage.
// Source: can1357/oh-my-pi docs/extensions.md and docs/hooks.md at
// 21f1188cf88b2f534c092181968cd5a4d1a9c296.
func init() {
	Register(Agent{Name: "omp", Adapter: piAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "Shared Pi bridge v1; tool_call uses block/reason with verified local shell metadata; plugin wiring pending"},
		WriteObservation: Support{Level: Limited, Detail: "Successful tool_result write/edit parameters; normalized edit paths require version-specific plugin verification"},
		SessionStart:     Support{Level: Limited, Detail: "session_start requires stable sessionManager identity; snapshot taken even when context delivery is unavailable"},
		Stop:             Support{Level: Limited, Detail: "session_stop is main-session idle; agent_end is observation-only; both map to silent Stop"},
		SessionEnd:       Support{Level: Limited, Detail: "session_shutdown has a 2-second parallel budget; silent bridge result and background handoff required"},
		Context:          Support{Level: Limited, Detail: "tool_result.additionalContext can deliver passive notices; plugin delivery pending"},
		MCP:              Support{Level: Limited, Detail: "Native mcp.json; default profile only in this phase; Pi/OMP shared paths need ownership receipts during installation"},
		Skill:            Support{Level: Unsupported, Detail: "Skill installation and ownership are not implemented for this agent"},
	}})
}
