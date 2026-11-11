package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
)

// Targets MoonshotAI/kimi-cli 9ab1286b (Beta hooks), not the separate kimi-code
// protocol. See hooks/events.py, hooks/runner.py and soul/toolset.py there.
type kimiAdapter struct{}

func (kimiAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var in claudeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("kimi event conflicts with invocation")
	}
	ev := Event{Name: in.Event, HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd, Source: in.Source}
	switch in.Event {
	case "PreToolUse", "PostToolUse", "SessionStart", "Stop", "SessionEnd":
	default:
		return Event{}, ErrSkip
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession("kimi:" + in.SessionID)
	}
	if in.Event == "PreToolUse" {
		if in.ToolName == "Shell" {
			ev.Command = firstString(in.ToolInput, "command")
		}
		// kaos can select another runtime; local Path.cwd() in the hook input
		// does not attest that Shell executes there (soul/toolset.py).
		ev.SkipReason = "kimi: tool runtime locality is not confirmed"
		return ev, nil
	}
	if ev.Session == "" {
		return Event{}, fmt.Errorf("kimi payload lacks a stable session id")
	}
	native, ok := NativePath(in.Cwd)
	if !ok || !filepath.IsAbs(native) {
		return Event{}, ErrSkip
	}
	ev.Cwd = native
	if in.Event == "PostToolUse" {
		// At the pinned revision this also fires for ToolError, passes only
		// str(ret)[:2000], and discards hook results in a background task.
		// The docs' "successful execution" wording is insufficient evidence.
		return ev, ErrSkip
	}
	if in.Event == "SessionStart" {
		// The reference describes stdout context, but this revision has no
		// verified delivery path. Preserve notices until that path is wired.
		ev.DeferContext = true
	}
	return ev, nil
}

func (kimiAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.Name == "PreToolUse" {
		return (traeAdapter{}).Encode(ev, result) // same documented deny schema
	}
	return nil, nil
}

func (kimiAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "kimi", Adapter: kimiAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Detail: "Beta hook payload does not attest the kaos tool runtime locality"},
		WriteObservation: Support{Detail: "kimi-cli 9ab1286b PostToolUse includes ToolError and discards hook output"},
		SessionStart:     Support{Level: Limited, Detail: "Beta event takes a local snapshot without acknowledging notices"},
		Stop:             Support{Level: Limited, Detail: "Beta hook; silent background finish using the session snapshot when available"},
		SessionEnd:       Support{Level: Limited, Detail: "Beta hook schema; silent final finish"},
		Context:          Support{Detail: "Context output is disabled; notices remain unacknowledged"},
		MCP:              Support{Level: Limited, Detail: "kimi-cli ~/.kimi/mcp.json; project configs require an explicit launch flag"},
		Skill:            Support{Level: Limited, Detail: "kimi-cli ~/.kimi/skills, independent of KIMI_SHARE_DIR"},
	}})
}
