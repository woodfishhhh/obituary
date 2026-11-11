package hook

import (
	"encoding/json"
	"io"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

// claudeInput is the part of a Claude Code hook payload Obituary reads.
type claudeInput struct {
	SessionID string         `json:"session_id"`
	Cwd       string         `json:"cwd"`
	Event     string         `json:"hook_event_name"`
	Source    string         `json:"source"` // SessionStart: startup, resume, clear, compact, fork
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type claudeOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		AdditionalContext        string `json:"additionalContext,omitempty"`
		PermissionDecision       string `json:"permissionDecision,omitempty"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

// ClaudeEvents are the events `obit init --agent claude` registers, with matchers.
var ClaudeEvents = []struct{ Event, Matcher string }{
	{"SessionStart", ""},
	// MultiEdit is retained for older clients; current tools use Write/Edit.
	{"PostToolUse", "Write|Edit|MultiEdit|NotebookEdit"},
	{"Stop", ""},
	{"SessionEnd", ""},
}

// Claude handles one Claude Code hook event read from in. It writes JSON to
// out only when the agent should hear something, and never returns an error
// to its caller: failures are logged and the hook exits quietly.
func Claude(e *auto.Engine, in io.Reader, out io.Writer, runner *Runner) {
	Run("claude", e, in, out, runner)
}

type claudeAdapter struct{}

func (claudeAdapter) Decode(raw []byte, _ Invocation) (Event, error) {
	var in claudeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	ev := Event{Name: in.Event, Tool: in.ToolName, Session: core.NormalizeSession(in.SessionID), Cwd: in.Cwd, Source: in.Source}
	ev.References, ev.Writing = fileReferences(in.ToolName, in.ToolInput)
	if in.Event == "PreToolUse" {
		switch in.ToolName {
		case "Bash":
			ev.Command = firstString(in.ToolInput, "command")
			ev.Shell = guardrm.Bash
		case "PowerShell":
			ev.Command = firstString(in.ToolInput, "command")
			ev.Shell = guardrm.PowerShell
		default:
			ev.SkipReason = "unsupported tool: " + in.ToolName
		}
	}
	if path := firstString(in.ToolInput, "file_path", "notebook_path"); path != "" {
		ev.Paths = []string{path}
	}
	return ev, nil
}

func (claudeAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if !result.Deny && result.Context == "" {
		return nil, nil
	}
	var o claudeOutput
	o.HookSpecificOutput.HookEventName = ev.Name
	// code.claude.com/docs/en/hooks#pretooluse-decision-control permits
	// additionalContext without a permission decision; retain host permissions.
	if ev.Name == "PreToolUse" && result.Deny {
		o.HookSpecificOutput.PermissionDecision = "deny"
		o.HookSpecificOutput.PermissionDecisionReason = result.Reason
	} else {
		o.HookSpecificOutput.AdditionalContext = result.Context
	}
	return EncodeJSON(o)
}

func (claudeAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "claude", Adapter: claudeAdapter{}, Capabilities: Capabilities{
		Target:           Target{Host: "Claude Code", Distribution: "CLI / Windows", Version: ">=2.1.139 (exec-form hooks)", Scope: "user/project"},
		DeleteGuard:      Support{Level: Supported, Detail: "Bash and PowerShell tools; opt-in with --guard-rm"},
		WriteObservation: Support{Level: Supported}, SessionStart: Support{Level: Supported},
		Stop: Support{Level: Supported}, SessionEnd: Support{Level: Supported},
		Context: Support{Level: Supported, Detail: "SessionStart and PostToolUse; ending events stay silent"},
		MCP:     Support{Level: Supported}, Skill: Support{Level: Supported},
	}})
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
