package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
)

// MiniMax-AI/minimax-code 3c21163c: plugin-hooks/{runner,wire-tool-adapter}.ts
// and local-runtime-v2's plugin-hook-tool-lifecycle.ts define this envelope.
// Translate native write.path to the shared event's paths before handling it.
type minimaxAdapter struct{}

func (minimaxAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in struct {
		declaredShellInput
		Response json.RawMessage `json:"tool_response"`
		IsError  *bool           `json:"is_error"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("minimax-code: conflicting event names")
	}
	ev := Event{Name: in.Event, HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd, Source: in.Source}
	switch in.Event {
	case "PreToolUse", "PostToolUse", "SessionStart", "Stop", "SessionEnd":
	default:
		return Event{}, ErrSkip
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession("minimax-code:" + in.SessionID)
	}
	if ev.Name == "PreToolUse" {
		switch in.ToolName {
		case "bash", "Bash", "PowerShell":
			in.declareShell(&ev)
		default:
			ev.SkipReason = "unsupported tool: " + in.ToolName
		}
		return ev, nil
	}
	// Windows-native callbacks only: mapping /mnt paths here could observe a
	// local namesake of a cloud/SSH file. The host envelope has no backend flag.
	if ev.Session == "" || !filepath.IsAbs(in.Cwd) || strings.HasPrefix(in.Cwd, "/") {
		return Event{}, ErrSkip
	}
	native, ok := NativePath(in.Cwd)
	if !ok {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if ev.Name == "PostToolUse" {
		if in.IsError != nil && *in.IsError || !minimaxWriteSucceeded(in.ToolName, in.Response) {
			return Event{}, ErrSkip
		}
		key := "file_path"
		switch in.ToolName {
		case "write":
			key = "path"
		case "edit", "Write", "Edit":
		default:
			return Event{}, ErrSkip
		}
		path := firstString(in.ToolInput, key)
		if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~") ||
			strings.Contains(path, ":") && !windowsCwd(path) || strings.HasPrefix(path, `\`) && !filepath.IsAbs(path) {
			return Event{}, ErrSkip
		}
		if path, ok := minimaxObservedPath(path, ev.Cwd, in.Response); ok {
			ev.Paths = []string{path}
		} else {
			return Event{}, ErrSkip
		}
	}
	return ev, nil
}

// MiniMax 3c21163c utils/paths.ts normalizes these spaces and one leading @.
// Native results need not contain the actual path: skip rewritten inputs then.
// Compatible results supply filePath; require it to match the host resolution.
// Home, URL and non-native spellings are rejected by Decode, not guessed here.
func minimaxObservedPath(input, cwd string, raw json.RawMessage) (string, bool) {
	normalized := strings.Map(func(r rune) rune {
		if r == '\u00a0' || r >= '\u2000' && r <= '\u200a' || r == '\u202f' || r == '\u205f' || r == '\u3000' {
			return ' '
		}
		return r
	}, input)
	normalized = strings.TrimPrefix(normalized, "@")
	var response struct {
		FilePath string `json:"filePath"`
	}
	if json.Unmarshal(raw, &response) != nil || normalized != input && response.FilePath == "" {
		return "", false
	}
	if normalized == "" || strings.HasPrefix(normalized, "~") || strings.HasPrefix(normalized, "/") {
		return "", false
	}
	native, ok := NativePath(normalized)
	if !ok {
		return "", false
	}
	if !filepath.IsAbs(native) {
		native = filepath.Join(cwd, native)
	}
	if response.FilePath != "" {
		actual, ok := NativePath(response.FilePath)
		if !ok || strings.HasPrefix(response.FilePath, "/") || !filepath.IsAbs(actual) || filepath.Clean(actual) != filepath.Clean(native) {
			return "", false
		}
		return actual, true
	}
	return native, true
}

func minimaxWriteSucceeded(tool string, raw json.RawMessage) bool {
	var response struct {
		IsError *bool             `json:"isError"`
		Error   *bool             `json:"is_error"`
		Content []json.RawMessage `json:"content"`
		Details *struct {
			IsError      *bool   `json:"is_error"`
			OK           *bool   `json:"ok"`
			Created      *bool   `json:"created"`
			BytesWritten *uint64 `json:"bytes_written"`
			Diff         *string `json:"diff"`
		} `json:"details"`
		Type            string            `json:"type"`
		FilePath        string            `json:"filePath"`
		StructuredPatch []json.RawMessage `json:"structuredPatch"`
	}
	if json.Unmarshal(raw, &response) != nil || response.IsError != nil && *response.IsError || response.Error != nil && *response.Error {
		return false
	}
	if d := response.Details; d != nil {
		if d.IsError != nil && *d.IsError || d.OK != nil && !*d.OK || len(response.Content) == 0 {
			return false
		}
		// The native wire drops top-level is_error. Require source-owned write
		// captures or edit diff metadata; generic output text cannot prove success.
		switch tool {
		case "write":
			return d.Created != nil && d.BytesWritten != nil
		case "edit":
			return d.Diff != nil
		}
	}
	// Compatible tool responses are emitted only after a captured successful
	// mutation. MiniMax may retain native names when adaptation is lossy.
	switch tool {
	case "Write":
		return (response.Type == "create" || response.Type == "update") && response.FilePath != "" && response.StructuredPatch != nil
	case "Edit":
		return response.FilePath != "" && response.StructuredPatch != nil
	}
	return false
}

func (minimaxAdapter) Encode(ev Event, result Result) ([]byte, error) {
	switch ev.Name {
	case "PreToolUse":
		if !result.Deny {
			return nil, nil
		}
		result.Context = ""
		result.Reason = capContext(result.Reason)
	case "SessionStart", "PostToolUse":
		if ev.DeferContext || result.Context == "" {
			return nil, nil
		}
		result.Deny, result.Reason = false, ""
		result.Context = capContext(result.Context)
	default:
		return nil, nil
	}
	return (claudeAdapter{}).Encode(ev, result)
}

func (minimaxAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "minimax-code", Adapter: minimaxAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "Requires agent_shells.minimax-code = bash or powershell confirming local Windows execution; undeclared commands and remote payloads pass through; native hooks require global installation"},
		WriteObservation: Support{Level: Limited, Detail: "Local Windows write/edit with captured success metadata; rewritten input needs a matching absolute result filePath; cloud/SSH/WSL paths skipped"},
		SessionStart:     Support{Level: Limited, Detail: "mcode 3c21163c SessionStart; local Windows workspaces only"},
		Stop:             Support{Level: Limited, Detail: "Silent turn finish; local Windows only"},
		SessionEnd:       Support{Level: Limited, Detail: "Silent final finish; local Windows only"},
		Context:          Support{Level: Limited, Detail: "mcode 3c21163c consumes Claude JSON additionalContext at SessionStart/PostToolUse"},
		MCP:              Support{Level: Limited, Detail: "Project .mcp.json requires host approval; global native MCP plugin requires a PATH interpreter"},
		Skill:            Support{Level: Limited, Detail: "Native local plugin skills; explicit plugin enable required"},
	}})
}
