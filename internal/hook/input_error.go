package hook

import (
	"bytes"
	"encoding/json"
	"regexp"
)

var (
	inputEventHint = regexp.MustCompile(`"hook_event_name"\s*:\s*("(?:[^"\\]|\\.)*")`)
	inputUsername  = regexp.MustCompile(`(?i)([\\/]+(?:Users|home)[\\/]+)[^\\/"\r\n]+`)
)

// LogInputError keeps invalid JSON diagnostics off the host protocol. The event
// hint is best-effort text from malformed input, never used to dispatch a hook.
func LogInputError(home string, invocation Invocation, data []byte, err error) {
	event := invocation.Event
	raw := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if event == "" {
		if match := inputEventHint.FindSubmatch(raw); match != nil {
			_ = json.Unmarshal(match[1], &event)
		}
	}
	if event == "" {
		event = "unknown"
	}
	event = event[:min(len(event), 80)]
	if len(raw) <= MaxInput && !json.Valid(raw) {
		// Limit before redaction, including a username cut off at byte 80.
		prefix := inputUsername.ReplaceAll(raw[:min(len(raw), 80)], []byte("${1}<user>"))
		Logf(home, "%s: input: %q; event_hint=%q input_prefix=%q", invocation.Agent, err, event, prefix)
		return
	}
	Logf(home, "%s: input: %q; event_hint=%q", invocation.Agent, err, event)
}
