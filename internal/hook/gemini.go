package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// geminicli.com/docs/hooks/reference/ and google-gemini/gemini-cli
// packages/core/src/tools/{write-file,edit,shell}.ts define this protocol.
// A Windows shell or sandbox backend requires evidence beyond the
// run_shell_command tool name.
type geminiAdapter struct{}

func (geminiAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in struct {
		Event    string         `json:"hook_event_name"`
		Session  string         `json:"session_id"`
		Cwd      string         `json:"cwd"`
		Source   string         `json:"source"`
		Tool     string         `json:"tool_name"`
		Input    map[string]any `json:"tool_input"`
		Response *struct {
			Content json.RawMessage `json:"llmContent"`
			Error   json.RawMessage `json:"error"`
		} `json:"tool_response"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && in.Event != invocation.Event {
		return Event{}, fmt.Errorf("gemini: conflicting event names")
	}
	ev := Event{HostEvent: in.Event, Cwd: in.Cwd, Source: in.Source, Tool: in.Tool}
	switch in.Event {
	case "BeforeTool":
		ev.Name = "PreToolUse"
	case "AfterTool":
		ev.Name = "PostToolUse"
	case "SessionStart", "SessionEnd":
		ev.Name = in.Event
	case "AfterAgent":
		ev.Name = "Stop"
	default:
		return Event{}, ErrSkip
	}
	if in.Session != "" {
		ev.Session = core.NormalizeSession("gemini:" + in.Session)
	}
	if ev.Name == "PreToolUse" {
		ev.Command = firstString(in.Input, "command")
		ev.SkipReason = "gemini: tool shell and sandbox backend are not confirmed by hook input"
		return ev, nil
	}
	native, ok := NativePath(ev.Cwd)
	if !ok || !filepath.IsAbs(native) || ev.Session == "" {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if ev.Name == "PostToolUse" {
		if in.Response == nil || len(in.Response.Content) == 0 || bytes.Equal(in.Response.Content, []byte("null")) ||
			(len(in.Response.Error) != 0 && !bytes.Equal(in.Response.Error, []byte("null"))) {
			return Event{}, ErrSkip
		}
		if in.Tool != "write_file" && in.Tool != "replace" {
			return Event{}, ErrSkip
		}
		path, ok := NativePath(firstString(in.Input, "file_path"))
		if !ok {
			return Event{}, ErrSkip
		}
		ev.Paths = []string{path}
	}
	return ev, nil
}

func (geminiAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.Name == "PreToolUse" && result.Deny {
		return EncodeJSON(struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}{"deny", result.Reason})
	}
	if !ev.DeferContext && (ev.Name == "SessionStart" || ev.Name == "PostToolUse") && result.Context != "" {
		var out claudeOutput
		out.HookSpecificOutput.HookEventName = ev.HostEvent
		out.HookSpecificOutput.AdditionalContext = capContext(result.Context)
		return EncodeJSON(out)
	}
	return []byte("{}\n"), nil
}

func (geminiAdapter) Fallback(Event) []byte { return []byte("{}\n") }

func init() {
	Register(Agent{Name: "gemini", Adapter: geminiAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Detail: "run_shell_command does not expose actual shell/backend; no pre hook should be installed"},
		WriteObservation: Support{Level: Limited, Detail: "write_file/replace with a tool_response and no error"},
		SessionStart:     Support{Level: Supported}, Stop: Support{Level: Supported},
		SessionEnd: Support{Level: Limited, Detail: "Best effort; host does not await completion"},
		Context:    Support{Level: Supported, Detail: "SessionStart/AfterTool hookSpecificOutput.additionalContext"},
		MCP:        Support{Detail: "Installation is a separate phase"}, Skill: Support{Detail: "Installation is a separate phase"},
	}})
}
