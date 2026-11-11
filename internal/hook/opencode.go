package hook

// OpenCode's V1 tool.execute.* and V2 execute.* callbacks are translated by
// the plugin into bridge v1. Shell and local-filesystem evidence belong to that
// plugin; tool names alone do not identify the execution environment.
// Sources: anomalyco/opencode packages/core/src/tool.ts and plugin/hooks.ts at
// 894e30395c49f65fc6256902eee8f887698da7d3.
type opencodeAdapter struct{ BridgeAdapter }

func init() {
	Register(Agent{Name: "opencode", Adapter: opencodeAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "Bridge v1 only; V1 tool.execute.before / V2 execute.before require verified local shell metadata; plugin denial uses Tool.Error"},
		WriteObservation: Support{Level: Limited, Detail: "Successful write/edit/apply_patch parameters only; plugin path extraction and context delivery remain to be wired"},
		SessionStart:     Support{Level: Limited, Detail: "session.created snapshots normally; observation-only callbacks leave notices pending"},
		Stop:             Support{Level: Limited, Detail: "session.idle maps to silent Stop; plugin lifecycle wiring pending"},
		SessionEnd:       Support{Level: Limited, Detail: "session.deleted is persistent-session deletion, not process exit; ordinary cleanup uses Stop"},
		Context:          Support{Level: Limited, Detail: "Bridge context_delivery must reflect a model-visible channel; pending notices delivered by the next eligible callback; plugin wiring pending"},
		MCP:              Support{Level: Limited, Detail: "Official local mcp entries in JSON/JSONC; installation belongs to phase two"},
		Skill:            Support{Level: Unsupported, Detail: "Skill installation and ownership are not implemented for this agent"},
	}})
}
