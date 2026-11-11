// capture is an opt-in protocol probe: it denies the marker-producing command.
package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		return
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || os.WriteFile(os.Args[1], data, 0o600) != nil {
		return
	}
	var input struct {
		ToolInput map[string]any `json:"tool_input"`
	}
	if json.Unmarshal(data, &input) != nil {
		return
	}
	command, _ := input.ToolInput["command"].(string)
	if !strings.Contains(command, "OBIT_PROTOCOL_EXECUTED") {
		return
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "PreToolUse", "permissionDecision": "deny",
		"permissionDecisionReason": "OBIT_DENY_REASON_VISIBLE: the protocol probe denied this tool call.",
	}})
}
