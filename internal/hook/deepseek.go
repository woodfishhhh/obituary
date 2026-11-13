package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// deepseekAdapter targets deepseek-ai/deepseek-harness fcc0ad46. The official
// Claude bridge preserves native tool names but drops result.isError. Successful
// writes therefore use our small tools/result observer, leaving context delivery
// and pre-tool denial with the official bridge (see testdata/deepseek-sources.json).
type deepseekAdapter struct{}

func (deepseekAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Event{}, err
	}
	if envelope.Version != 0 {
		return deepseekBridgeEvent(raw, invocation)
	}
	var in claudeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if invocation.Event != "" && invocation.Event != in.Event {
		return Event{}, fmt.Errorf("deepseek-harness: conflicting event names")
	}
	ev := Event{Name: in.Event, HostEvent: in.Event, Tool: in.ToolName, Cwd: in.Cwd, Source: in.Source}
	switch in.Event {
	case "PreToolUse", "PostToolUse", "SessionStart", "UserPromptSubmit", "Stop":
	default:
		return Event{}, ErrSkip
	}
	if in.SessionID != "" {
		ev.Session = core.NormalizeSession("deepseek-harness:" + in.SessionID)
	}
	local := deepseekLocal() && deepseekWindowsPath(in.Cwd)
	if in.Event == "PreToolUse" {
		return deepseekPreToolEvent(ev, in, local), nil
	}
	if !local || ev.Session == "" {
		return Event{}, ErrSkip
	}
	switch in.Event {
	case "PostToolUse":
		// The official bridge flattens successes and failures to strings. No
		// output text or model-supplied path can establish a successful write.
		ev.Name = "Context"
	case "UserPromptSubmit":
		ev.Name = "Context"
	case "SessionStart":
		ev.DeferContext = true // detached best effort; notices wait for an awaited callback
	}
	return ev, nil
}

func deepseekBridgeEvent(raw []byte, invocation Invocation) (Event, error) {
	var in BridgeRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		return Event{}, err
	}
	if in.Event != "PostToolUse" || in.HostEvent != "tools/result" || (in.Tool != "write" && in.Tool != "edit") ||
		!deepseekLocal() || !in.Local || len(in.Paths) != 1 || !deepseekWindowsPath(in.Cwd) || !deepseekWindowsPath(in.Paths[0]) {
		return Event{}, ErrSkip
	}
	ev, err := (BridgeAdapter{}).Decode(raw, invocation)
	ev.DeferContext = true // tools/result is observation-only; the official bridge drains later.
	return ev, err
}

func deepseekPreToolEvent(ev Event, in claudeInput, local bool) Event {
	ev.Command = firstString(in.ToolInput, "command")
	switch in.ToolName {
	case "bash":
		ev.Shell = guardrm.Bash
	case "pwsh":
		ev.Shell = guardrm.PowerShell
	default:
		ev.SkipReason = "deepseek-harness: unsupported native shell tool"
	}
	// Both tools consume the replaceable ctx.shell service. Its dialect and
	// locality need operator attestation, independently of the tool name.
	if !local || string(ev.Shell) != os.Getenv("OBIT_DEEPSEEK_SHELL") {
		ev.SkipReason = "deepseek-harness: local backend and shell are not confirmed"
	}
	if workdir, exists := in.ToolInput["workdir"]; exists {
		path, ok := workdir.(string)
		if !ok || path == "" || runtime.GOOS == "windows" && strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) || strings.Contains(path, ":") && !deepseekWindowsPath(path) {
			ev.SkipReason = "deepseek-harness: unsupported workdir"
		} else if filepath.IsAbs(path) {
			ev.Cwd = path
		} else {
			ev.Cwd = filepath.Join(in.Cwd, path)
		}
	}
	return ev
}

func deepseekLocal() bool { return os.Getenv("OBIT_DEEPSEEK_LOCAL") == "1" }

func deepseekWindowsPath(path string) bool {
	if runtime.GOOS != "windows" {
		return filepath.IsAbs(path) && !strings.ContainsAny(path, "\x00\r\n") && pathutil.Unchanged(path) == nil
	}
	return windowsCwd(path) && !strings.ContainsAny(path, "\x00\r\n") && pathutil.Unchanged(filepath.FromSlash(path)) == nil
}

func (deepseekAdapter) Encode(ev Event, result Result) ([]byte, error) {
	if ev.HostEvent == "tools/result" {
		return (BridgeAdapter{}).Encode(ev, Result{})
	}
	if ev.Name == "Context" {
		if ev.DeferContext || result.Context == "" {
			return nil, nil
		}
		var out claudeOutput
		out.HookSpecificOutput.HookEventName = ev.HostEvent
		out.HookSpecificOutput.AdditionalContext = capContext(result.Context)
		return EncodeJSON(out)
	}
	return (traeAdapter{}).Encode(ev, result)
}

func (deepseekAdapter) Fallback(Event) []byte { return nil }

func init() {
	Register(Agent{Name: "deepseek-harness", Adapter: deepseekAdapter{}, Capabilities: Capabilities{
		DeleteGuard:      Support{Level: Limited, Detail: "fcc0ad46 official Claude bridge; native bash/pwsh only after OBIT_DEEPSEEK_LOCAL=1 and OBIT_DEEPSEEK_SHELL=bash or powershell attest the actual local backend"},
		WriteObservation: Support{Level: Limited, Detail: "Native tools/result observer requires isError=false, write/edit result.value.path and same-host filesystem mapping; official PostToolUse is context-only"},
		SessionStart:     Support{Level: Limited, Detail: "Official bridge is detached/best effort; snapshot can miss the first write; context deferred"},
		Stop:             Support{Level: Limited, Detail: "Silent turn-stopping hook; local backend attestation required"},
		SessionEnd:       Support{Detail: "Official bridge has no SessionEnd event"},
		Context:          Support{Level: Limited, Detail: "Awaited UserPromptSubmit/PostToolUse deliver pending notices; local backend attestation required"},
		MCP:              Support{Level: Limited, Detail: "Owned dsh-mcp-client overlay; explicit --patch or profile bundle activation required"},
		Skill:            Support{Level: Limited, Detail: "DSH_HOME/skills or project .dsh/skills with dsh-skill-filesystem default roots enabled"},
	}})
}
