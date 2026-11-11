//go:build windows

package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Most adapters use their published JSON captures. Claude/Codex's older writes
// live in the Go fuzz corpus; read those exact payloads without duplicating them.
func lifecyclePayload(t *testing.T, agent string, pre bool, repo string) (map[string]any, string) {
	t.Helper()
	files := map[string][2]string{
		"antigravity":      {"antigravity-pre", "antigravity-post"},
		"claude":           {"", "fuzz/FuzzClaudePayload/write"},
		"codex":            {"codex-local-shell", "fuzz/FuzzCodexPayload/patch"},
		"cline":            {"cline-pre", "cline-post"},
		"codebuddy":        {"codebuddy-cli-pre", "codebuddy-cli-post"},
		"codebuddy-ide":    {"codebuddy-ide-pre", "codebuddy-ide-post"},
		"copilot":          {"copilot-cli-pre", "copilot-cli-post"},
		"cursor":           {"cursor-pre", "cursor-edit"},
		"deepseek-harness": {"deepseek-bash-pre", "deepseek-write-post"},
		"droid":            {"droid-pre", "droid-write-unverified"},
		"gemini":           {"gemini-pre", "gemini-write"},
		"hermes":           {"hermes-pre", "hermes-post"},
		"kilocode":         {"kilocode-pre", "kilocode-post"},
		"kimi":             {"kimi-pre", "kimi-post"},
		"lingma":           {"lingma-pre", "lingma-post"},
		"minimax-code":     {"minimax-native-pre", "minimax-post"},
		"omp":              {"omp-protocol", "omp-protocol"},
		"openclaw":         {"openclaw-protocol", "openclaw-protocol"},
		"opencode":         {"opencode-protocol", "opencode-protocol"},
		"pi":               {"pi-protocol", "pi-protocol"},
		"qoder":            {"qoder-cli-pre", "qoder-post-feedback"},
		"qoder-cn":         {"qoder-cn-pre", "qoder-cn-post"},
		"trae":             {"trae-pre", "trae-post"},
		"trae-cn":          {"trae-pre", "trae-post"},
		"vibe":             {"vibe-pre", "vibe-post"},
		"zcode":            {"zcode-pre", "zcode-post"},
	}
	pair, ok := files[agent]
	if !ok {
		t.Fatalf("new registered adapter %q needs protocol fixture coverage", agent)
	}
	index := 1
	if pre {
		index = 0
	}
	file := pair[index]
	if file == "" { // Claude PreToolUse uses the same envelope as its write fixture.
		file = pair[1]
	}
	if !strings.HasPrefix(file, "fuzz/") {
		file += ".json"
	}
	// commandRepo changes cwd to the disposable project; recover the source root
	// from the caller's source file instead of using the current directory.
	data, err := os.ReadFile(filepath.Join(lifecycleSourceDir(), "..", "hook", "testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(file, "fuzz/") {
		line := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[1])
		text, err := strconv.Unquote(strings.TrimSuffix(strings.TrimPrefix(line, "[]byte("), ")"))
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(text)
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	if cases, ok := input["cases"].([]any); ok {
		want := "PostToolUse"
		if pre {
			want = "PreToolUse"
		}
		input = nil
		for _, item := range cases {
			bridge := item.(map[string]any)["bridge"].(map[string]any)
			if bridge["event"] == want {
				input = bridge
				break
			}
		}
		if input == nil {
			t.Fatalf("fixture %s lacks %s", file, want)
		}
	}
	input = lifecycleMapStrings(input, func(value string) string {
		for _, prefix := range []string{"C:/isolated/repo", "C:/work/repo", "C:/second", "C:/first", "C:/repo", `C:\repo`, "/mnt/d/repo"} {
			value = strings.ReplaceAll(value, prefix, filepath.ToSlash(repo))
		}
		return strings.ReplaceAll(value, "app.log", "debug.log")
	}).(map[string]any)
	event := ""
	if agent == "antigravity" {
		event = "PostToolUse"
		if pre {
			event = "PreToolUse"
		}
	}
	if agent == "copilot" {
		event = "postToolUse"
		if pre {
			event = "preToolUse"
		}
	}
	if pre {
		const command = "rm -rf build"
		input = lifecycleMapCommands(input, command).(map[string]any)
		switch agent {
		case "claude":
			input["hook_event_name"], input["tool_name"] = "PreToolUse", "Bash"
			input["tool_input"] = map[string]any{"command": command}
		case "copilot":
			input["toolName"] = "bash"
			input["toolArgs"] = string(lifecycleJSON(t, map[string]string{"command": command}))
		}
		if _, bridge := input["version"]; bridge {
			input["shell"] = "bash"
		}
	} else if agent == "deepseek-harness" {
		// The official bridge capture deliberately lacks success evidence.
		// Reproduce the installed tools/result observer's Bridge v1 projection
		// using its captured input path (plugins/deepseek/index.ts).
		input = map[string]any{
			"version": 1, "agent": agent, "event": "PostToolUse", "host_event": "tools/result", "tool": "write",
			"session": input["session_id"], "cwd": input["cwd"], "paths": []any{input["tool_input"].(map[string]any)["file_path"]},
			"local": true, "success": true, "context_delivery": false,
		}
	}
	return input, event
}

// Keep the host's actual path fields, including Codex's patch headers, while
// making the local namesake explicit. Relative paths cannot expose cwd bypasses.
func lifecycleAbsoluteWrite(t *testing.T, input map[string]any, repo string) map[string]any {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(repo, "debug.log"))
	return lifecycleMapStrings(input, func(value string) string {
		if value == "debug.log" {
			return path
		}
		if strings.Contains(value, "*** Begin Patch") {
			for _, header := range []string{"*** Add File: ", "*** Update File: ", "*** Move to: "} {
				value = strings.ReplaceAll(value, header+"debug.log", header+path)
			}
		}
		return value
	}).(map[string]any)
}

func lifecycleSourceDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func lifecycleMapStrings(value any, change func(string) string) any {
	switch value := value.(type) {
	case string:
		return change(value)
	case map[string]any:
		for key, item := range value {
			value[key] = lifecycleMapStrings(item, change)
		}
	case []any:
		for i, item := range value {
			value[i] = lifecycleMapStrings(item, change)
		}
	}
	return value
}

func lifecycleMapCommands(value any, command string) any {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if key == "command" || key == "cmd" || key == "CommandLine" {
				if argv, ok := item.([]any); ok {
					argv[len(argv)-1] = command
				} else {
					value[key] = command
				}
			} else {
				value[key] = lifecycleMapCommands(item, command)
			}
		}
	}
	return value
}
