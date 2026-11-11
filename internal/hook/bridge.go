package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

const BridgeVersion = 1

// BridgeRequest is the versioned stdin protocol for a thin TS/Python plugin.
// Event is neutral; HostEvent and Tool retain the host's original names. Local,
// Shell, Success and Paths must come from verified tool/runtime metadata. Tool
// output strings are never path evidence. ContextDelivery means this callback can
// deliver returned context to the model; observation-only callbacks leave it false.
type BridgeRequest struct {
	Version         int           `json:"version"`
	Agent           string        `json:"agent"`
	Event           string        `json:"event"`
	HostEvent       string        `json:"host_event,omitempty"`
	Tool            string        `json:"tool,omitempty"`
	Session         string        `json:"session,omitempty"`
	Cwd             string        `json:"cwd,omitempty"`
	Source          string        `json:"source,omitempty"`
	Command         string        `json:"command,omitempty"`
	Shell           guardrm.Shell `json:"shell,omitempty"`
	Local           bool          `json:"local"`
	Success         *bool         `json:"success,omitempty"`
	Paths           []string      `json:"paths,omitempty"`
	ContextDelivery bool          `json:"context_delivery"`
	SkipReason      string        `json:"skip_reason,omitempty"`
}

// BridgeResponse never grants permission or rewrites a command. An empty result
// is the versioned neutral reply; Stop and SessionEnd always produce that reply.
type BridgeResponse struct {
	Version int `json:"version"`
	Result
}

// BridgeAdapter is embedded by plugin agents whose input already follows this
// protocol. Each agent still registers its own name and verified capabilities.
type BridgeAdapter struct{}

func (BridgeAdapter) Decode(raw []byte, invocation Invocation) (Event, error) {
	var input BridgeRequest
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), &input); err != nil {
		return Event{}, err
	}
	if input.Version != BridgeVersion {
		return Event{}, fmt.Errorf("unsupported bridge version %d", input.Version)
	}
	if !validAgentName(invocation.Agent) || input.Agent != invocation.Agent {
		return Event{}, fmt.Errorf("bridge agent %q does not match invocation %q", input.Agent, invocation.Agent)
	}
	switch input.Event {
	case "PreToolUse", "PostToolUse", "SessionStart", "Context", "Stop", "SessionEnd":
	default:
		return Event{}, fmt.Errorf("unsupported bridge event %q", input.Event)
	}
	if invocation.Event != "" && invocation.Event != input.Event {
		return Event{}, fmt.Errorf("bridge event conflicts with invocation")
	}
	ev := Event{Name: input.Event, HostEvent: input.HostEvent, Tool: input.Tool, Cwd: input.Cwd,
		Source: input.Source, Command: input.Command, Shell: input.Shell, SkipReason: input.SkipReason,
		DeferContext: !input.ContextDelivery}
	if input.Session != "" {
		ev.Session = core.NormalizeSession(input.Agent + ":" + input.Session)
	}
	native, reachable := NativePath(input.Cwd)
	if !input.Local || !reachable || !filepath.IsAbs(native) {
		if ev.Name != "PreToolUse" {
			return Event{}, fmt.Errorf("bridge requires a confirmed local absolute working directory")
		}
		ev.SkipReason = "bridge: local filesystem not confirmed"
	}
	if ev.Name == "PreToolUse" {
		if input.Local && reachable && filepath.IsAbs(native) {
			for _, path := range input.Paths {
				if p, ok := NativePath(path); ok {
					ev.References = append(ev.References, p)
				}
			}
		}
		if ev.Shell != guardrm.Bash && ev.Shell != guardrm.PowerShell {
			ev.SkipReason = "bridge: shell syntax not confirmed"
		}
		return ev, nil
	}
	if ev.SkipReason != "" {
		return Event{}, fmt.Errorf("bridge event skipped: %s", ev.SkipReason)
	}
	if ev.Session == "" {
		return Event{}, fmt.Errorf("bridge payload lacks a stable session id")
	}
	ev.Cwd = native
	if ev.Name == "PostToolUse" {
		if input.Success == nil || !*input.Success {
			return Event{}, fmt.Errorf("bridge write success not confirmed")
		}
		for _, path := range input.Paths {
			if native, ok := NativePath(path); ok {
				ev.Paths = append(ev.Paths, native)
			}
		}
	}
	return ev, nil
}

func (BridgeAdapter) Encode(ev Event, result Result) ([]byte, error) {
	switch ev.Name {
	case "Stop", "SessionEnd":
		result = Result{}
	case "PreToolUse":
		result.Context = ""
		if !result.Deny {
			result.Reason = ""
		}
	default:
		result.Deny, result.Reason = false, ""
		if ev.DeferContext {
			result.Context = ""
		}
	}
	return EncodeJSON(BridgeResponse{Version: BridgeVersion, Result: result})
}

func (BridgeAdapter) Fallback(Event) []byte { return []byte("{\"version\":1}\n") }
