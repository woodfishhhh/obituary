package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// CodeBuddy's CLI and Craft IDE share Claude's response envelope, but retain
// their native tool names and argument keys. Sources: codebuddy-sources.json.
// Keep host translation outside Handle so all adapters share its decisions.
type codeBuddyAdapter struct{ name string }

func (a codeBuddyAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in *struct {
		declaredShellInput
		Response json.RawMessage `json:"tool_response"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if in == nil {
		return Event{}, fmt.Errorf("codebuddy: expected an object")
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("codebuddy: conflicting event names")
	}
	ev := Event{Name: in.Event, HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd, Source: in.Source}
	switch in.Event {
	case "PreToolUse", "PostToolUse", "SessionStart", "Stop", "SessionEnd":
	case "UserPromptSubmit":
		ev.Name = "Context"
	default:
		return Event{}, ErrSkip
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession(a.name + ":" + in.SessionID)
	}
	if ev.Name == "PreToolUse" {
		switch in.ToolName {
		case "Bash", "PowerShell", "execute_command":
			in.declareShell(&ev)
		default:
			ev.SkipReason = "unsupported tool: " + in.ToolName
		}
		return ev, nil
	}
	if ev.Session == "" {
		return Event{}, fmt.Errorf("codebuddy: missing session id")
	}
	cwd, ok := NativePath(in.Cwd)
	if !ok || !filepath.IsAbs(cwd) {
		return Event{}, ErrSkip
	}
	ev.Cwd = cwd
	if ev.Name != "PostToolUse" {
		return ev, nil
	}
	// The CLI reference documents success:true. IDE tool_response is any;
	// only this explicit success envelope is supported, never output text.
	var response struct {
		Success *bool           `json:"success"`
		Error   json.RawMessage `json:"error"`
		IsError bool            `json:"is_error"`
	}
	if err := json.Unmarshal(in.Response, &response); err != nil {
		return Event{}, ErrSkip
	}
	if response.Success == nil || !*response.Success || response.IsError ||
		(len(response.Error) > 0 && !bytes.Equal(bytes.TrimSpace(response.Error), []byte("null"))) {
		return Event{}, ErrSkip
	}
	var path string
	switch in.ToolName {
	case "Write", "Edit", "MultiEdit":
		path = firstString(in.ToolInput, "file_path")
	case "write_to_file", "replace_in_file":
		camel, snake := firstString(in.ToolInput, "filePath"), firstString(in.ToolInput, "file_path")
		if camel != "" && snake != "" && camel != snake {
			return Event{}, fmt.Errorf("codebuddy: conflicting filePath and file_path")
		}
		path = firstString(in.ToolInput, "filePath", "file_path")
	default:
		return Event{}, ErrSkip
	}
	path, ok = NativePath(path)
	if !ok || bytes.Contains([]byte(path), []byte("://")) {
		return Event{}, ErrSkip
	}
	ev.Paths = []string{path}
	return ev, nil
}

func (codeBuddyAdapter) Encode(ev Event, result Result) ([]byte, error) {
	switch ev.Name {
	case "PreToolUse":
		if !result.Deny {
			return nil, nil
		}
		result.Context, result.Reason = "", capContext(result.Reason)
	case "SessionStart", "PostToolUse", "Context":
		if ev.DeferContext || result.Context == "" {
			return nil, nil
		}
		result = Result{Context: capContext(result.Context)}
		if ev.Name == "Context" {
			ev.Name = "UserPromptSubmit"
		}
	default:
		return nil, nil // Stop / SessionEnd must never continue the host turn.
	}
	return (claudeAdapter{}).Encode(ev, result)
}

func (codeBuddyAdapter) Fallback(Event) []byte { return nil }

func init() {
	for _, name := range []string{"codebuddy", "codebuddy-ide"} {
		write := "CLI Write/Edit/MultiEdit file_path with explicit tool_response.success:true; native Windows paths only"
		if name == "codebuddy-ide" {
			write = "Craft write_to_file/replace_in_file filePath/file_path only with explicit success:true; other IDE result envelopes unverified"
		}
		Register(Agent{Name: name, Adapter: codeBuddyAdapter{name: name}, Capabilities: Capabilities{
			DeleteGuard:      Support{Level: Limited, Detail: "Requires agent_shells." + name + " = bash or powershell confirming local Windows execution; undeclared commands and remote payloads pass through; host not run"},
			WriteObservation: Support{Level: Limited, Detail: write},
			SessionStart:     Support{Level: Limited, Detail: "Documented local SessionStart; synthetic protocol fixtures, host not run"},
			Stop:             Support{Level: Supported, Detail: "Silent Stop"}, SessionEnd: Support{Level: Limited, Detail: "Documented SessionEnd; host shutdown timing unverified"},
			Context: Support{Level: Limited, Detail: "Documented hookSpecificOutput.additionalContext on SessionStart/PostToolUse/UserPromptSubmit; host not run"},
			MCP:     Support{Level: Limited, Detail: "CLI first-existing JSONC MCP; legacy runtime state and IDE/plugin profile targets require manual selection"},
			Skill:   Support{Level: Limited, Detail: ".codebuddy/skills with shared owner receipts; host trust/enablement remains manual"},
		}})
	}
}
