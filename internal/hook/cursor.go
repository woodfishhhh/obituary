package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// cursor.com/docs/hooks documents the flat event envelope. Responses use the
// event-specific schema while leaving tool input and permission grants to the host.
type cursorAdapter struct{}

func (cursorAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in struct {
		Event        string         `json:"hook_event_name"`
		Conversation string         `json:"conversation_id"`
		Cwd          string         `json:"cwd"`
		Roots        []string       `json:"workspace_roots"`
		Background   bool           `json:"is_background_agent"`
		Tool         string         `json:"tool_name"`
		Command      string         `json:"command"`
		Input        map[string]any `json:"tool_input"`
		Path         string         `json:"file_path"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && in.Event != invocation.Event {
		return Event{}, fmt.Errorf("cursor: conflicting event names")
	}
	ev := Event{HostEvent: in.Event, Cwd: in.Cwd, Tool: in.Tool}
	switch in.Event {
	case "preToolUse", "beforeShellExecution":
		ev.Name = "PreToolUse"
		// Shell/command do not identify the tool's shell on Windows. A
		// user-supplied shell key is not documented runtime evidence either.
		ev.Command = firstString(in.Input, "command")
		if in.Event == "beforeShellExecution" {
			ev.Command = in.Command
		}
		ev.SkipReason = "cursor: actual shell and local execution are not supplied by the hook protocol"
	case "afterFileEdit":
		ev.Name, ev.DeferContext = "PostToolUse", true
	case "sessionStart":
		ev.Name, ev.DeferContext = "SessionStart", true
	case "postToolUse":
		// Only deliver pending context. The generic Write input schema is
		// unverified; afterFileEdit remains the source of file observations.
		ev.Name = "Context"
	case "stop":
		ev.Name = "Stop"
	case "sessionEnd":
		ev.Name = "SessionEnd"
	default:
		// postToolUse is successful, but the published Write tool_input
		// schema is not fixed. afterFileEdit supplies the documented path.
		return Event{}, ErrSkip
	}
	if in.Conversation != "" {
		ev.Session = core.NormalizeSession("cursor:" + in.Conversation)
	}
	if ev.Name == "PreToolUse" {
		return ev, nil
	}
	if in.Background || ev.Session == "" {
		return Event{}, ErrSkip
	}
	if ev.Cwd == "" && len(in.Roots) == 1 {
		ev.Cwd = in.Roots[0]
	}
	native, ok := NativePath(ev.Cwd)
	if !ok || !filepath.IsAbs(native) {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if ev.Name == "PostToolUse" {
		path, ok := NativePath(in.Path)
		if !ok || !filepath.IsAbs(path) {
			return Event{}, ErrSkip
		}
		ev.Paths = []string{path}
	}
	return ev, nil
}

func (cursorAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.Name == "PreToolUse" && result.Deny {
		return EncodeJSON(struct {
			Permission string `json:"permission"`
			Message    string `json:"agent_message"`
		}{"deny", result.Reason})
	}
	if !ev.DeferContext && (ev.Name == "SessionStart" || ev.Name == "PostToolUse" || ev.Name == "Context") && result.Context != "" {
		return EncodeJSON(struct {
			Context string `json:"additional_context"`
		}{capContext(result.Context)})
	}
	return nil, nil
}

// The official default failClosed=false accepts no output. Invalid JSON/schema
// can block even with that default, and an explicit allow could grant permission.
// Do not install this adapter with failClosed=true (there is no verified shell).
func (cursorAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "cursor", Adapter: cursorAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Detail: "Shell has no trustworthy shell/locality metadata; no pre hook should be installed"},
		WriteObservation: Support{Level: Limited, Detail: "afterFileEdit observes local paths; postToolUse only delivers pending context, without inferring write paths"},
		SessionStart:     Support{Level: Limited, Detail: "Fire-and-forget callback; snapshot can race first write; local foreground, one root or explicit cwd"},
		Stop:             Support{Level: Supported}, SessionEnd: Support{Level: Supported},
		Context: Support{Level: Limited, Detail: "Pending notices delivered through postToolUse additional_context; asynchronous sessionStart and afterFileEdit leave them unacknowledged"},
		MCP:     Support{Detail: "Installation is a separate phase"}, Skill: Support{Detail: "Installation is a separate phase"},
	}})
}
