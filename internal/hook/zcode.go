package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/woodfishhhh/obituary/internal/agentid"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// ZCode a9fe401e's configured-runner-input.ts adds lossless snake_case aliases
// to its native camelCase contract. Keep that contract separate from Claude's
// decoder; only the verified response format and patch reader are reused.
type zcodeAdapter struct{}

type zcodeInput struct {
	claudeInput
	Response json.RawMessage `json:"tool_response"`
	Error    json.RawMessage `json:"error"`
}

func normalizeZCodeAliases(raw []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for _, pair := range [][2]string{
		{"hookEventName", "hook_event_name"}, {"sessionId", "session_id"},
		{"toolName", "tool_name"}, {"toolInput", "tool_input"}, {"toolResponse", "tool_response"},
		{"toolCallId", "tool_use_id"}, {"agentName", "agent_type"}, {"mode", "permission_mode"},
		{"stopHookActive", "stop_hook_active"}, {"transcriptPath", "transcript_path"},
	} {
		native, n := fields[pair[0]]
		alias, a := fields[pair[1]]
		if n && a {
			decode := func(data []byte) any {
				var value any
				d := json.NewDecoder(bytes.NewReader(data))
				d.UseNumber()
				_ = d.Decode(&value) // Unmarshal above already validated the JSON.
				return value
			}
			if !reflect.DeepEqual(decode(native), decode(alias)) {
				return nil, fmt.Errorf("zcode: conflicting %s and %s", pair[0], pair[1])
			}
		}
		if n {
			fields[pair[1]] = native
		}
	}
	return json.Marshal(fields)
}

func (zcodeAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	data, err := normalizeZCodeAliases(raw)
	if err != nil {
		return Event{}, err
	}
	var in zcodeInput
	if err := json.Unmarshal(data, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("zcode: event conflicts with invocation")
	}
	ev := Event{Name: in.Event, HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd, Source: in.Source}
	switch in.Event {
	case "PreToolUse", "PostToolUse", "SessionStart", "Stop":
	case "UserPromptSubmit":
		ev.Name = "Context"
	default:
		return Event{}, ErrSkip // Includes failed tools and the nonexistent SessionEnd.
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession(agentid.ZCode + ":" + in.SessionID)
	}
	if ev.Name == "PreToolUse" {
		if in.ToolName == "Bash" {
			ev.Command = firstString(in.ToolInput, "command")
		}
		// docs/bash-shell-provider.md: Windows Bash may run Git Bash, CMD or
		// legacy-shell. The payload has no selected shell/backend attestation.
		ev.SkipReason = "zcode: actual Bash shell and local execution backend are not confirmed"
		return ev, nil
	}
	native, ok := NativePath(in.Cwd)
	if !ok || !filepath.IsAbs(native) || ev.Session == "" {
		return Event{}, ErrSkip
	}
	ev.Cwd, err = pathutil.Abs(native)
	if err != nil {
		return Event{}, err
	}
	if ev.Name != "PostToolUse" {
		return ev, nil
	}
	return zcodeWriteEvent(ev, in)
}

func zcodeWriteEvent(ev Event, in zcodeInput) (Event, error) {
	// hook-flow.ts emits Post only after successful execution and serialization.
	// A missing response or an explicit error cannot attest a successful write.
	if len(in.Response) == 0 || bytes.Equal(in.Response, []byte("null")) ||
		len(in.Error) != 0 && !bytes.Equal(in.Error, []byte("null")) {
		return Event{}, ErrSkip
	}
	var paths []string
	var err error
	switch in.ToolName {
	case "Write", "Edit":
		path, ok := NativePath(firstString(in.ToolInput, "file_path"))
		if !ok || !filepath.IsAbs(path) {
			return Event{}, ErrSkip
		}
		paths = []string{path}
	case "ApplyPatch":
		paths, err = localPatchPaths(firstString(in.ToolInput, "patch_text"))
		if err != nil {
			return Event{}, err
		}
	default:
		return Event{}, ErrSkip
	}
	for _, path := range paths {
		native, ok := NativePath(path)
		if !ok {
			return Event{}, ErrSkip
		}
		if !filepath.IsAbs(native) {
			native = filepath.Join(ev.Cwd, native)
		}
		native, err = pathutil.Abs(native)
		if err != nil {
			return Event{}, err
		}
		ev.Paths = append(ev.Paths, native)
	}
	if len(ev.Paths) == 0 {
		return Event{}, ErrSkip
	}
	return ev, nil
}

func (zcodeAdapter) Encode(ev Event, result Result) ([]byte, error) {
	switch ev.Name {
	case "PreToolUse":
		if !result.Deny {
			return nil, nil
		}
		result = Result{Deny: true, Reason: capContext(result.Reason)}
	case "SessionStart", "PostToolUse", "Context":
		if ev.DeferContext || result.Context == "" {
			return nil, nil
		}
		result = Result{Context: capContext(result.Context)}
		if ev.HostEvent != "" {
			ev.Name = ev.HostEvent
		}
	default:
		return nil, nil // Stop must never continue the turn.
	}
	return (claudeAdapter{}).Encode(ev, result)
}

func (zcodeAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: agentid.ZCode, Adapter: zcodeAdapter{}, Capabilities: Capabilities{
		Target:           Target{Host: "ZCode", Distribution: "standalone CLI / local Windows", Version: "a9fe401eede6e6e5e01be9048679c206fe6f02d9", Scope: "user/project"},
		DeleteGuard:      Support{Level: Limited, Detail: "Claude JSON deny supported; Bash may use Git Bash, CMD or legacy shell without payload attestation, so no PreToolUse hook installed; use obit rm/bury explicitly"},
		WriteObservation: Support{Level: Supported, Detail: "Successful Write/Edit file_path and ApplyPatch patch_text; local Windows paths"},
		SessionStart:     Support{Level: Limited, Detail: "Project hooks need digest-bound review/grant; skipped SessionStart is not replayed, create a new session after approval"},
		Stop:             Support{Level: Supported, Detail: "Silent background finish"},
		SessionEnd:       Support{Detail: "Not in ZCode's seven-event protocol"},
		Context:          Support{Level: Supported, Detail: "SessionStart/UserPromptSubmit and successful PostToolUse additionalContext"},
		MCP:              Support{Level: Limited, Detail: "mcp.servers plus features.mcp; user server names override project names"},
		Skill:            Support{Level: Limited, Detail: "User/project .zcode/skills; features.skill and skills.enabled plus per-skill overrides control loading; verify with zcode skills list --cwd <project> --json in a new process"},
	}})
}
