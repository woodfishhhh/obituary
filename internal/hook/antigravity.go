package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
)

// https://antigravity.google/docs/hooks (2026-11-09). The hook event is passed
// by the installed command's --event flag, not guessed from toolCall presence.
type antigravityAdapter struct{}

type antigravityInput struct {
	Session    string   `json:"conversationId"`
	Workspaces []string `json:"workspacePaths"`
	ToolCall   struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	} `json:"toolCall"`
	Error string `json:"error"`
}

func (antigravityAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in antigravityInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	ev := Event{Name: invocation.Event, HostEvent: invocation.Event, Tool: in.ToolCall.Name}
	switch ev.Name {
	case "PreToolUse", "PostToolUse", "SessionStart", "Stop":
	case "PreInvocation":
		ev.Name = "Context"
	default:
		return Event{}, ErrSkip
	}
	if in.Session != "" {
		ev.Session = core.NormalizeSession("antigravity:" + in.Session)
	}
	if ev.Name == "PreToolUse" {
		if ev.Tool == "run_command" {
			ev.Command = firstString(in.ToolCall.Args, "CommandLine")
			ev.Cwd = firstString(in.ToolCall.Args, "Cwd")
		}
		// Neither the CLI/App/IDE executor nor a remote runtime is identified
		// by this payload. Do not turn a generic run_command into Bash evidence.
		ev.SkipReason = "antigravity: local execution shell is not confirmed"
		return ev, nil
	}
	if ev.Session == "" {
		return Event{}, fmt.Errorf("antigravity payload lacks a stable conversation id")
	}
	if ev.Name == "PostToolUse" {
		return antigravityWriteEvent(ev, in)
	}
	// One Event cannot snapshot/finish multiple workspace roots. Reject that
	// ambiguity rather than silently covering only the first project.
	if len(in.Workspaces) != 1 {
		return ev, ErrSkip
	}
	root, ok := NativePath(in.Workspaces[0])
	if !ok || !filepath.IsAbs(root) {
		return ev, ErrSkip
	}
	ev.Cwd = root
	return ev, nil
}

func antigravityWriteEvent(ev Event, in antigravityInput) (Event, error) {
	ev.DeferContext = true // host ignores stdout; RunEvent must not acknowledge notices
	if in.Error != "" {
		return ev, ErrSkip
	}
	switch ev.Tool {
	case "write_to_file", "replace_file_content", "multi_replace_file_content":
	default:
		return ev, ErrSkip
	}
	path, ok := NativePath(firstString(in.ToolCall.Args, "TargetFile"))
	if !ok || !filepath.IsAbs(path) {
		return ev, ErrSkip
	}
	// Pick the workspace containing the explicit target, never [0]. A
	// lexical selection is only routing; core still verifies each object.
	for _, workspace := range in.Workspaces {
		root, ok := NativePath(workspace)
		if !ok || !filepath.IsAbs(root) {
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) && len(root) > len(ev.Cwd) {
			ev.Cwd = root
		}
	}
	if ev.Cwd == "" {
		return ev, ErrSkip
	}
	ev.Paths = []string{path}
	return ev, nil
}

func (antigravityAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.Name == "PreToolUse" && result.Deny {
		return EncodeJSON(map[string]string{"decision": "deny", "reason": capContext(result.Reason)})
	}
	if (ev.Name == "SessionStart" || ev.Name == "Context") && !ev.DeferContext && result.Context != "" {
		return EncodeJSON(map[string]any{"injectSteps": []map[string]string{{"ephemeralMessage": capContext(result.Context)}}})
	}
	return (antigravityAdapter{}).Fallback(ev), nil
}

// Antigravity's "allow" means defer to other hooks and the default policy,
// unlike "auto_approve". PreToolUse requires a decision; other events accept {}.
func (antigravityAdapter) Fallback(ev Event) []byte {
	if ev.Name == "PreToolUse" {
		return []byte("{\"decision\":\"allow\"}\n")
	}
	return []byte("{}\n")
}

func init() {
	Register(Agent{Name: "antigravity", Adapter: antigravityAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Detail: "Payload does not attest local execution shell across CLI/App/IDE"},
		WriteObservation: Support{Level: Limited, Detail: "Successful TargetFile writes observed; notices wait for SessionStart or PreInvocation"},
		SessionStart:     Support{Level: Limited, Detail: "Exactly one local workspace"},
		Stop:             Support{Level: Limited, Detail: "Exactly one local workspace; neutral output"},
		SessionEnd:       Support{Detail: "No native SessionEnd event"},
		Context:          Support{Level: Limited, Detail: "SessionStart/PreInvocation injectSteps in one local workspace; PostToolUse output ignored"},
		MCP:              Support{Level: Supported}, Skill: Support{Detail: "Plugin rules supported; Skill installation is not implemented for this agent"},
	}})
}
