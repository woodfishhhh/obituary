package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// docs.factory.com/harness/hooks defines the Claude-shaped envelope. The
// execution backend must be verified separately from the Execute tool name.
type droidAdapter struct{}

func (droidAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in struct {
		Event   string         `json:"hook_event_name"`
		Session string         `json:"session_id"`
		Cwd     string         `json:"cwd"`
		Source  string         `json:"source"`
		Tool    string         `json:"tool_name"`
		Input   map[string]any `json:"tool_input"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && in.Event != invocation.Event {
		return Event{}, fmt.Errorf("droid: conflicting event names")
	}
	ev := Event{HostEvent: in.Event, Cwd: in.Cwd, Source: in.Source, Tool: in.Tool}
	switch in.Event {
	case "PreToolUse", "SessionStart", "Stop", "SessionEnd":
		ev.Name = in.Event
	case "PostToolUse":
		// The official reference says "after a tool completes", not only
		// successful calls, and leaves tool_response tool-dependent. Until
		// those success schemas are verified, do not register failed writes.
		return Event{}, ErrSkip
	default:
		return Event{}, ErrSkip
	}
	if in.Session != "" {
		ev.Session = core.NormalizeSession("droid:" + in.Session)
	}
	if ev.Name == "PreToolUse" {
		ev.Command = firstString(in.Input, "command")
		ev.SkipReason = "droid: Execute does not identify the actual shell/backend"
		return ev, nil
	}
	native, ok := NativePath(ev.Cwd)
	if !ok || !filepath.IsAbs(native) || ev.Session == "" {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	return ev, nil
}

func (droidAdapter) Encode(ev Event, result Result) ([]byte, error) {
	var out claudeOutput
	out.HookSpecificOutput.HookEventName = ev.HostEvent
	if ev.Name == "PreToolUse" && result.Deny {
		out.HookSpecificOutput.PermissionDecision = "deny"
		out.HookSpecificOutput.PermissionDecisionReason = result.Reason
	} else if !ev.DeferContext && (ev.Name == "PostToolUse" || ev.Name == "SessionStart") && result.Context != "" {
		out.HookSpecificOutput.AdditionalContext = capContext(result.Context)
	} else {
		return nil, nil
	}
	return EncodeJSON(out)
}

func (droidAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "droid", Adapter: droidAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Detail: "Execute does not identify the actual shell/backend; no pre hook should be installed"},
		WriteObservation: Support{Detail: "Official tool_response success shapes for Create/Edit/ApplyPatch are not fixed; PostToolUse skipped"},
		SessionStart:     Support{Level: Supported}, Stop: Support{Level: Supported}, SessionEnd: Support{Level: Supported},
		Context: Support{Level: Limited, Detail: "SessionStart only; PostToolUse skipped"},
		MCP:     Support{Detail: "Installation is a separate phase"}, Skill: Support{Detail: "Installation is a separate phase"},
	}})
}
