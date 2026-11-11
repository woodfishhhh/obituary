package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

// Copilot has two envelopes, selected by the configured event name. See
// docs.github.com/en/copilot/reference/hooks-reference. Responses may deny
// or add context; permission grants and tool input remain under host control.
type copilotAdapter struct{}

type copilotInput struct {
	Event         string          `json:"hook_event_name"`
	Session       *string         `json:"session_id"`
	NativeSession *string         `json:"sessionId"`
	Cwd           string          `json:"cwd"`
	Source        string          `json:"source"`
	Tool          *string         `json:"tool_name"`
	NativeTool    *string         `json:"toolName"`
	Input         json.RawMessage `json:"tool_input"`
	NativeInput   json.RawMessage `json:"toolArgs"`
	Result        *struct {
		Type string `json:"result_type"`
	} `json:"tool_result"`
	NativeResult *struct {
		Type string `json:"resultType"`
	} `json:"toolResult"`
}

func (copilotAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in *copilotInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if in == nil {
		return Event{}, fmt.Errorf("copilot: expected an object")
	}
	host := in.Event
	if host == "" {
		host = invocation.Event
	}
	if invocation.Event != "" && invocation.Event != host {
		return Event{}, fmt.Errorf("copilot: conflicting event names")
	}
	ev, compat, err := copilotEvent(in, host)
	if err != nil {
		return Event{}, err
	}
	ev, input, success, err := copilotEnvelope(ev, in, compat)
	if err != nil {
		return Event{}, err
	}
	native, reachable := NativePath(ev.Cwd)
	local := reachable && filepath.IsAbs(native)
	if ev.Name == "PreToolUse" {
		ev.Command = firstString(input, "command")
		// PascalCase maps BOTH bash and powershell to Bash. VS Code's
		// run_in_terminal also omits its actual shell. Never guess from either.
		switch {
		case !local:
			ev.SkipReason = "copilot: local working directory not confirmed"
		case !compat && ev.Tool == "bash":
			ev.Shell = guardrm.Bash
		case !compat && ev.Tool == "powershell":
			ev.Shell = guardrm.PowerShell
		default:
			ev.SkipReason = "copilot: shell syntax not confirmed for " + ev.Tool
		}
		return ev, nil
	}
	if ev.Session == "" || !local {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if ev.Name == "PostToolUse" {
		return copilotWriteEvent(ev, input, success)
	}
	return ev, nil
}

func copilotEvent(in *copilotInput, host string) (Event, bool, error) {
	ev := Event{HostEvent: host, Cwd: in.Cwd, Source: in.Source}
	compat := false
	switch host {
	case "PreToolUse", "PostToolUse", "SessionStart", "Stop", "SessionEnd":
		compat, ev.Name = true, host
	case "preToolUse":
		ev.Name = "PreToolUse"
	case "postToolUse":
		ev.Name = "PostToolUse"
	case "sessionStart":
		ev.Name = "SessionStart"
	case "agentStop":
		ev.Name = "Stop"
	case "sessionEnd":
		ev.Name = "SessionEnd"
	default:
		return Event{}, false, ErrSkip
	}
	return ev, compat, nil
}

func copilotEnvelope(ev Event, in *copilotInput, compat bool) (Event, map[string]any, bool, error) {
	session, tool, args := in.NativeSession, in.NativeTool, in.NativeInput
	success := in.NativeResult != nil && in.NativeResult.Type == "success"
	if compat {
		if in.NativeSession != nil || in.NativeTool != nil || len(in.NativeInput) != 0 || in.NativeResult != nil {
			return Event{}, nil, false, fmt.Errorf("copilot: mixed envelopes")
		}
		session, tool, args = in.Session, in.Tool, in.Input
		success = in.Result != nil && in.Result.Type == "success"
	} else if in.Session != nil || in.Tool != nil || len(in.Input) != 0 || in.Result != nil {
		return Event{}, nil, false, fmt.Errorf("copilot: mixed envelopes")
	}
	if session != nil && *session != "" {
		ev.Session = core.NormalizeSession("copilot:" + *session)
	}
	if tool != nil {
		ev.Tool = *tool
	}
	input, err := copilotArguments(args)
	if err != nil {
		return Event{}, nil, false, err
	}
	return ev, input, success, nil
}

func copilotWriteEvent(ev Event, input map[string]any, success bool) (Event, error) {
	if !success {
		return Event{}, ErrSkip
	}
	// Compatible names retain the runtime tool's original arguments.
	// Do not interpret shell result text or generic path-looking keys.
	var path string
	switch ev.Tool {
	case "create", "edit":
		path = firstString(input, "path")
	case "Write", "Edit":
		path = firstString(input, "file_path", "path")
	default:
		return Event{}, ErrSkip
	}
	if path, ok := NativePath(path); ok {
		ev.Paths = []string{path}
	} else {
		return Event{}, ErrSkip
	}
	return ev, nil
}

func copilotArguments(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		raw = []byte(text)
	}
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("copilot tool arguments: %w", err)
	}
	return input, nil
}

func (copilotAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.Name == "PreToolUse" && result.Deny {
		if ev.HostEvent == "PreToolUse" {
			var out claudeOutput
			out.HookSpecificOutput.HookEventName = ev.HostEvent
			out.HookSpecificOutput.PermissionDecision = "deny"
			out.HookSpecificOutput.PermissionDecisionReason = result.Reason
			return EncodeJSON(out)
		}
		return EncodeJSON(struct {
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		}{"deny", result.Reason})
	}
	// Current CLI/VS Code harness consumes top-level additionalContext for
	// both event spellings. Stop/SessionEnd must never create another turn.
	if !ev.DeferContext && (ev.Name == "PostToolUse" || ev.Name == "SessionStart") && result.Context != "" {
		return EncodeJSON(struct {
			Context string `json:"additionalContext"`
		}{capContext(result.Context)})
	}
	return nil, nil
}

func (copilotAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "copilot", Adapter: copilotAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "Native bash/powershell only; compatible Bash and VS Code run_in_terminal have no reliable shell identity"},
		WriteObservation: Support{Level: Limited, Detail: "Successful create/edit and compatible Write/Edit only"},
		SessionStart:     Support{Level: Supported}, Stop: Support{Level: Supported},
		SessionEnd: Support{Level: Limited, Detail: "CLI /clear only in the current harness; detached callback"},
		Context:    Support{Level: Limited, Detail: "Current CLI/VS Code harness top-level additionalContext"},
		MCP:        Support{Detail: "Installation is a separate phase"}, Skill: Support{Detail: "Installation is a separate phase"},
	}})
}
