package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// Native protocol: https://docs.mistral.ai/vibe/code/cli/hooks (2026-11-09).
// Tool arguments: mistralai/mistral-vibe c96a6dbb, vibe/core/tools/builtins/
// write_file.py and edit.py. Responses leave tool input and permission grants
// under host control.
type vibeAdapter struct{}

func (vibeAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in struct {
		claudeInput
		Status string `json:"tool_status"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("vibe event conflicts with invocation")
	}
	ev := Event{HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd}
	switch in.Event {
	case "pre_tool":
		ev.Name = "PreToolUse"
	case "post_tool":
		ev.Name = "PostToolUse"
	case "post_agent":
		ev.Name = "Stop"
	default:
		return Event{}, ErrSkip
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession("vibe:" + in.SessionID)
	}
	if ev.Name == "PreToolUse" {
		switch in.ToolName {
		case "bash", "file_system.bash", "process_start", "file_system.process_start":
			ev.Command = firstString(in.ToolInput, "command")
		}
		// Vibe chooses Git Bash or PowerShell on Windows. The native hook
		// payload exposes neither that choice nor a verified local executor.
		ev.SkipReason = "vibe: execution shell and locality are not confirmed"
		return ev, nil
	}
	if ev.Session == "" {
		return Event{}, fmt.Errorf("vibe payload lacks a stable session id")
	}
	native, ok := NativePath(in.Cwd)
	if !ok || !filepath.IsAbs(native) {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if ev.Name == "PostToolUse" {
		return vibeWriteEvent(ev, in.claudeInput, in.Status)
	}
	return ev, nil
}

func vibeWriteEvent(ev Event, in claudeInput, status string) (Event, error) {
	if status != "success" {
		return ev, ErrSkip
	}
	switch in.ToolName {
	case "write_file", "file_system.write_file", "edit", "file_system.edit":
	default:
		return ev, ErrSkip
	}
	path, ok := NativePath(firstString(in.ToolInput, "file_path"))
	if !ok || !filepath.IsAbs(path) {
		return ev, ErrSkip
	}
	ev.Paths = []string{path}
	return ev, nil
}

func (vibeAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.Name == "PreToolUse" && result.Deny {
		return EncodeJSON(struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}{"deny", capContext(result.Reason)})
	}
	if ev.Name == "PostToolUse" && !ev.DeferContext && result.Context != "" {
		return EncodeJSON(map[string]any{"hook_specific_output": map[string]string{"additional_context": capContext(result.Context)}})
	}
	return nil, nil
}

func (vibeAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "vibe", Adapter: vibeAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Detail: "Native payload does not identify Git Bash versus PowerShell or executor locality"},
		WriteObservation: Support{Level: Limited, Detail: "Successful local write_file/edit only; no shell output inference"},
		SessionStart:     Support{Detail: "No native start event; no pre-tool snapshot emulation"},
		Stop:             Support{Level: Limited, Detail: "post_agent is silent; no start snapshot for session differences"},
		SessionEnd:       Support{Detail: "No native end event"},
		Context:          Support{Level: Limited, Detail: "post_tool additional_context only"},
		MCP:              Support{Level: Supported}, Skill: Support{Level: Limited, Detail: "VIBE_HOME/skills; system prompts are not skills"},
	}})
}
