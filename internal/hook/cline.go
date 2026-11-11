package hook

import "fmt"

// The SDK plugin owns callback argument extraction and maps deny to skip/reason.
// Contract: docs.cline.bot/sdk/plugins (AgentPlugin, 3-second hook timeout).
type clineAdapter struct{ BridgeAdapter }

func (a clineAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	if invocation.Agent != "cline" {
		return Event{}, fmt.Errorf("Cline adapter requires the cline invocation") //nolint:staticcheck // ST1005: Cline is the host's proper name in hook diagnostics.
	}
	ev, err := a.BridgeAdapter.Decode(raw, invocation)
	if err != nil {
		return Event{}, err
	}
	var name string
	switch ev.HostEvent {
	case "beforeTool":
		name = "PreToolUse"
	case "afterTool":
		name = "PostToolUse"
	case "beforeRun":
		// The plugin emits this once per stable session, before its first run.
		name = "SessionStart"
	case "afterRun":
		name = "Stop"
	case "onDispose":
		name = "SessionEnd"
	default:
		return Event{}, ErrSkip
	}
	if ev.Name != name {
		return Event{}, fmt.Errorf("Cline %s requires %s", ev.HostEvent, name) //nolint:staticcheck // ST1005: Cline is the host's proper name in hook diagnostics.
	}
	if name == "PreToolUse" && ev.Tool != "run_commands" {
		ev.SkipReason = "Cline: unsupported shell tool"
	}
	if name == "PostToolUse" && ev.Tool != "editor" && ev.Tool != "edit_file" && ev.Tool != "apply_patch" {
		return Event{}, ErrSkip
	}
	return ev, nil
}

func init() {
	Register(Agent{Name: "cline", Adapter: clineAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "SDK/CLI/Kanban beforeTool skip/reason; OBIT_CLINE_LOCAL_SHELL=bash or powershell requires user-verified local executor; real host validation pending"},
		WriteObservation: Support{Level: Limited, Detail: "successful editor/edit_file/apply_patch with local executor assertion; VS Code/JetBrains plugins excluded"},
		SessionStart:     Support{Level: Limited, Detail: "beforeRun snapshot once per stable session; local executor assertion required"},
		Stop:             Support{Level: Limited, Detail: "afterRun maps to silent Stop"},
		SessionEnd:       Support{Level: Limited, Detail: "setup ctx.onDispose maps to silent SessionEnd"},
		Context:          Support{Level: Limited, Detail: "beforeRun/afterTool appendContext; SDK plugin runtime contract tested"},
		MCP:              Support{Level: Limited, Detail: "user data/settings/cline_mcp_settings.json; project MCP manual configuration"},
		Skill:            Support{Level: Unsupported, Detail: "managed Skill installation has not been verified"},
	}})
}
