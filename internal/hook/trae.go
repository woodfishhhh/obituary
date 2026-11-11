package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// Protocol: https://docs.trae.ai/ide/hook-configuration-reference (2026-11-09).
// Translate native JSON to shared events. Responses only deny or add context,
// leaving tool input and permission grants under host control.
type traeAdapter struct{ name string }

func (a traeAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in claudeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("trae event conflicts with invocation")
	}
	ev := Event{Name: in.Event, HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd, Source: in.Source}
	switch in.Event {
	case "PreToolUse", "PostToolUse", "SessionStart", "Stop":
	default:
		return Event{}, ErrSkip
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession(a.name + ":" + in.SessionID)
	}
	if in.Event == "PreToolUse" {
		if in.ToolName == "RunCommand" {
			ev.Command = firstString(in.ToolInput, "command")
		}
		// PowerShell is the documented hook launcher, not proof of the
		// RunCommand tool's shell. tool_input.shell is not a documented signal.
		ev.SkipReason = "trae: tool execution shell is not confirmed"
		return ev, nil
	}
	if ev.Session == "" {
		return Event{}, fmt.Errorf("trae payload lacks a stable session id")
	}
	native, ok := NativePath(in.Cwd)
	if !ok || !filepath.IsAbs(native) {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if in.Event == "PostToolUse" {
		// The reference defines tool_response only as an object and lists
		// Write/Edit without their argument or success schema. Do not infer
		// Claude's file_path or success from the normalized tool name.
		return ev, ErrSkip
	}
	return ev, nil
}

func (traeAdapter) Encode(ev Event, result Result) ([]byte, error) {
	var out claudeOutput
	out.HookSpecificOutput.HookEventName = ev.Name
	switch ev.Name {
	case "PreToolUse":
		if !result.Deny {
			return nil, nil
		}
		out.HookSpecificOutput.PermissionDecision = "deny"
		out.HookSpecificOutput.PermissionDecisionReason = capContext(result.Reason)
	case "SessionStart", "PostToolUse":
		if result.Context == "" || ev.DeferContext {
			return nil, nil
		}
		out.HookSpecificOutput.AdditionalContext = capContext(result.Context)
	default:
		return nil, nil
	}
	return EncodeJSON(out)
}

func (traeAdapter) Fallback(Event) []byte { return nil }

func init() {
	for _, name := range []string{"trae", "trae-cn"} {
		Register(Agent{Name: name, Adapter: traeAdapter{name: name}, Capabilities: Capabilities{
			DeleteGuard:      Support{Detail: "RunCommand payload has no verified execution shell; passed through"},
			WriteObservation: Support{Detail: "Write/Edit success and path schema not documented; PostToolUse skipped"},
			SessionStart:     Support{Level: Supported}, Stop: Support{Level: Supported},
			SessionEnd: Support{Detail: "No documented SessionEnd event"},
			Context:    Support{Level: Limited, Detail: "SessionStart only; write observation is unavailable"},
			MCP:        Support{Level: Limited, Detail: "Project .trae/mcp.json requires project MCP enabled; global path unverified"},
			Skill:      Support{Detail: "Skill installation location not verified"},
		}})
	}
}
