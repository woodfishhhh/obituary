package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// The documented Claude envelope is shared by Qoder CLI, IDE/JB and CN.
// See testdata/qoder-sources.json. This adapter only translates protocol fields;
// host configuration stays outside decoding.
type qoderAdapter struct{ name string }

func (a qoderAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in claudeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("%s event conflicts with invocation", a.name)
	}
	ev := Event{Name: in.Event, HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd, Source: in.Source}
	switch in.Event {
	case "PreToolUse", "PostToolUse", "Stop":
	case "UserPromptSubmit":
		ev.Name = "Context"
	case "SessionStart", "SessionEnd":
		if a.name == "lingma" {
			return Event{}, ErrSkip
		}
	default:
		return Event{}, ErrSkip
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession(a.name + ":" + in.SessionID)
	}
	if in.Event == "PreToolUse" {
		if in.ToolName == "Bash" || in.ToolName == "run_in_terminal" {
			ev.Command = firstString(in.ToolInput, "command")
		}
		// Bash is also an IDE matcher alias. Neither it nor undocumented
		// tool_input.shell/local fields attest the actual shell or backend.
		ev.SkipReason = a.name + ": execution shell and local backend are not confirmed"
		return ev, nil
	}
	native, ok := NativePath(in.Cwd)
	if !ok || !filepath.IsAbs(native) || ev.Session == "" {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if in.Event == "PostToolUse" {
		return a.writeEvent(ev, in)
	}
	return ev, nil
}

func (a qoderAdapter) writeEvent(ev Event, in claudeInput) (Event, error) {
	switch in.ToolName {
	case "Write", "Edit", "create_file", "search_replace":
	case "edit_file":
		if a.name == "lingma" {
			return Event{}, ErrSkip
		}
	default:
		return Event{}, ErrSkip
	}
	path, ok := NativePath(firstString(in.ToolInput, "file_path"))
	if !ok || !filepath.IsAbs(path) {
		return Event{}, ErrSkip
	}
	path, err := pathutil.Abs(path)
	if err != nil {
		return Event{}, ErrSkip
	}
	ev.Paths = []string{path}
	// IDE Post feedback is user-visible, not model context. Compatible
	// names cannot distinguish IDE from CLI, so defer all variants until
	// UserPromptSubmit/SessionStart rather than acknowledge unseen text.
	ev.DeferContext = true
	return ev, nil
}

func (qoderAdapter) Encode(ev Event, result Result) ([]byte, error) {
	switch ev.Name {
	case "PreToolUse":
		if !result.Deny {
			return nil, nil
		}
		result = Result{Deny: true, Reason: capContext(result.Reason)}
	case "SessionStart", "Context":
		if ev.DeferContext || result.Context == "" {
			return nil, nil
		}
		result = Result{Context: capContext(result.Context)}
		if ev.HostEvent != "" {
			ev.Name = ev.HostEvent
		}
	default:
		return nil, nil // Post, Stop and SessionEnd never continue a turn.
	}
	return (claudeAdapter{}).Encode(ev, result)
}

func (qoderAdapter) Fallback(Event) []byte { return nil }

func qoderCapabilities() Capabilities {
	return Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "Claude JSON deny is documented; Bash/run_in_terminal do not attest shell/backend, so no guard is installed"},
		WriteObservation: Support{Level: Limited, Detail: "Documented successful PostToolUse with file_path for Write/Edit/create_file/search_replace/edit_file; local Windows paths only"},
		SessionStart:     Support{Level: Limited, Detail: "CLI and current 12-event IDE reference; older IDE versions may not emit SessionStart"},
		Stop:             Support{Level: Supported, Detail: "Silent turn finish"},
		SessionEnd:       Support{Level: Limited, Detail: "CLI/current IDE event; best effort on host termination"},
		Context:          Support{Level: Limited, Detail: "Post feedback is deferred to SessionStart/UserPromptSubmit"},
		MCP:              Support{Level: Limited, Detail: "CLI user settings.json and project .mcp.json; IDE/plugin MCP location requires its settings UI"},
		Skill:            Support{Detail: "Skill installation is not implemented for this agent"},
	}
}

func init() {
	for _, name := range []string{"qoder", "qoder-cn"} {
		Register(Agent{Name: name, Adapter: qoderAdapter{name: name}, Capabilities: qoderCapabilities()})
	}
}
