package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var claudeLookPath = exec.LookPath
var claudeMCPTimeout = 30 * time.Second
var claudeMCPWithTimeout = context.WithTimeout

type claudeMCPChange struct {
	before json.RawMessage
	want   json.RawMessage // nil for removal; includes user fields
}

func claudeEntry(data []byte) (json.RawMessage, error) {
	root, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	raw, ok := root.get("mcpServers")
	if !ok {
		return nil, nil
	}
	servers, err := parseObject(raw)
	if err != nil {
		return nil, err
	}
	entry, _ := servers.get("obituary")
	return entry, nil
}

func planClaudeMCP(ch Change, executable string, uninstall bool) (Change, error) {
	if !uninstall && !filepath.IsAbs(executable) {
		return ch, fmt.Errorf("user MCP for Claude requires an absolute obit.exe path: %q", executable)
	}
	planned, err := planMCPJSON(ch, executable, uninstall)
	if err != nil {
		return ch, err
	}
	before, err := claudeEntry(ch.orig)
	if err != nil {
		return ch, err
	}
	planned.claude = &claudeMCPChange{before: before}
	if planned.Action == "unchanged" {
		return planned, nil
	}
	if !uninstall {
		want, err := claudeEntry(planned.data)
		if err != nil {
			return ch, err
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, want); err != nil {
			return ch, err
		}
		planned.claude.want = compact.Bytes()
	}
	if len(before) != 0 {
		planned.Commands = append(planned.Commands, []string{"claude", "mcp", "remove", "--scope", "user", "obituary"})
	}
	if !uninstall {
		planned.Commands = append(planned.Commands, claudeAddJSON(planned.claude.want))
	}
	planned.data = nil // the state file is never written by Apply
	planned.Detail = "Claude user MCP via official CLI; requires >= 2.1.318 for configuration locking; " + ClaudeUpgradeHint
	if len(before) != 0 && !uninstall {
		planned.Detail += "; remove then add is not atomic"
	}
	return planned, nil
}

func claudeAddJSON(entry json.RawMessage) []string {
	return []string{"claude", "mcp", "add-json", "--scope", "user", "obituary", string(entry)}
}

// Official channel setting: https://code.claude.com/docs/en/setup#configure-release-channel
const ClaudeUpgradeHint = `the stable channel may lag below 2.1.318; choose latest in /config > Auto-update channel (or set "autoUpdatesChannel": "latest" in Claude settings.json), then run claude update`

// CommandText is a PowerShell invocation with literal, single-quoted argv.
func CommandText(args []string) string {
	note := ""
	if len(args) == 7 && args[2] == "add-json" {
		args, note = claudeManualArgs(args)
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "''") + "'"
	}
	return "& " + strings.Join(quoted, " ") + note
}

func claudeManualArgs(args []string) ([]string, string) {
	if manual, ok := claudeManualAdd(json.RawMessage(args[6])); ok {
		manual[0] = args[0]
		return manual, ""
	}
	args = slices.Clone(args)
	if !strings.EqualFold(filepath.Ext(args[0]), ".exe") {
		args[0] = "claude.exe"
	}
	note := " # Requires PowerShell 7.3 or newer, $PSNativeCommandArgumentPassing = 'Standard' or 'Windows'; use the native claude.exe (npm: node_modules/@anthropic-ai/claude-code/bin/claude.exe)"
	return args, note
}

// The official MCP docs specify repeatable -e KEY=value before another option
// (the env option is variadic). add avoids embedded JSON quotes in PowerShell 5.1.
// https://code.claude.com/docs/en/mcp#option-3-add-a-local-stdio-server
func claudeManualAdd(raw json.RawMessage) ([]string, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, false
	}
	for key := range fields {
		if key != "type" && key != "command" && key != "args" && key != "env" {
			return nil, false
		}
	}
	var entry struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	if json.Unmarshal(raw, &entry) != nil || entry.Command == "" || entry.Type != "" && entry.Type != "stdio" {
		return nil, false
	}
	args := []string{"claude", "mcp", "add", "--scope", "user"}
	keys := make([]string, 0, len(entry.Env))
	for key := range entry.Env {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		args = append(args, "-e", key+"="+entry.Env[key])
	}
	args = append(args, "--transport", "stdio", "obituary", "--", entry.Command)
	args = append(args, entry.Args...)
	// about_Parsing: Legacy drops empty arguments and embedded double quotes.
	// .cmd remains Legacy even under PowerShell 7.3's default Windows mode.
	// https://learn.microsoft.com/powershell/module/microsoft.powershell.core/about/about_parsing
	for _, arg := range args {
		if arg == "" || strings.ContainsRune(arg, '"') {
			return nil, false
		}
	}
	return args, true
}

func applyClaudeMCP(c Change) (applied []string, err error) {
	manual := make([]string, len(c.Commands))
	for i, args := range c.Commands {
		manual[i] = CommandText(args)
	}
	cli, err := resolveClaudeExecutable()
	if err != nil {
		return nil, fmt.Errorf("%w; close all Claude Code sessions, then run in PowerShell:\n%s", err, strings.Join(manual, "\n"))
	}
	out, err := runClaude(cli, []string{"--version"})
	if err != nil || !claudeHasConfigLock(out) {
		return nil, claudeConfigVersionError(out, err, manual)
	}
	// Recheck only obituary after the version probe; unrelated runtime saves
	// are expected. The official CLI merges while holding its own config lock.
	if err := checkOriginal(c); err != nil {
		return nil, err
	}
	for _, args := range c.Commands {
		out, runErr := runClaude(cli, args[1:]) // inherits CLAUDE_CONFIG_DIR
		state, readErr := observeClaudeMCP(c)
		if state == "new" {
			// Another normal installer may have finished while remove ran.
			return append(applied, "target state confirmed"), nil
		}
		expected := "new"
		if args[2] == "remove" {
			expected = "missing"
		}
		if state == expected {
			applied = append(applied, CommandText(args))
			// A committed final state is success even after timeout/WaitDelay.
			if args[2] != "remove" || len(c.claude.want) == 0 || runErr == nil {
				continue
			}
		}
		failure := claudeRegistrationError(fmt.Sprintf("%s: verification observed %s registration", CommandText(args), state), out, runErr, readErr)
		if state == "missing" && len(c.claude.before) != 0 && len(c.claude.want) != 0 {
			restored, recoveryErr := restoreClaudeMCP(c, cli)
			applied = append(applied, restored...)
			if recoveryErr == nil {
				return applied, nil
			}
			return applied, errors.Join(failure, recoveryErr)
		}
		if state == "unknown" {
			failure = errors.Join(failure, claudeRecoveryCommand(c, "cannot confirm registration; automatic recovery was not attempted"))
		}
		return applied, failure
	}
	return applied, nil
}

func restoreClaudeMCP(c Change, cli string) ([]string, error) {
	state, err := observeClaudeMCP(c)
	if state == "new" {
		return []string{"target state confirmed"}, nil
	}
	if state == "unknown" {
		return nil, errors.Join(err, claudeRecoveryCommand(c, "cannot confirm registration for recovery"))
	}
	if state == "old" {
		return nil, errors.New("old registration restored")
	}
	if state != "missing" {
		return nil, errors.New("registration is present after failure; preserved without recovery")
	}
	args := claudeAddJSON(c.claude.before)
	out, runErr := runClaude(cli, args[1:])
	state, err = observeClaudeMCP(c)
	if state == "new" {
		return []string{"target state confirmed"}, nil
	}
	if state == "old" {
		return []string{"restore " + CommandText(args)}, errors.New("old registration restored")
	}
	detail := "automatic recovery failed; registration " + state
	if state == "unknown" {
		detail = "cannot confirm registration after automatic recovery"
	}
	return nil, errors.Join(claudeRegistrationError(detail, out, runErr, err), claudeRecoveryCommand(c, detail))
}

func claudeConfigVersionError(out []byte, err error, manual []string) error {
	version, commands := strings.TrimSpace(string(out)), strings.Join(manual, "\n")
	if err == nil {
		return fmt.Errorf("configuration locking requires Claude Code >= 2.1.318 (version output %q, error <nil>); %s; or keep the current version, close all Claude Code sessions, then run in PowerShell:\n%s", version, ClaudeUpgradeHint, commands)
	}
	return fmt.Errorf("configuration locking requires Claude Code >= 2.1.318 (version output %q, error %w); %s; or keep the current version, close all Claude Code sessions, then run in PowerShell:\n%s", version, err, ClaudeUpgradeHint, commands)
}

// Keep diagnostics for absent causes unchanged while wrapping every real error.
func claudeRegistrationError(detail string, out []byte, runErr, readErr error) error {
	switch {
	case runErr == nil && readErr == nil:
		return fmt.Errorf("%s (process error: <nil>; read error: <nil>)\n%s", detail, out)
	case runErr == nil:
		return fmt.Errorf("%s (process error: <nil>; read error: %w)\n%s", detail, readErr, out)
	case readErr == nil:
		return fmt.Errorf("%s (process error: %w; read error: <nil>)\n%s", detail, runErr, out)
	default:
		return fmt.Errorf("%s (process error: %w; read error: %w)\n%s", detail, runErr, readErr, out)
	}
}

func claudeRecoveryCommand(c Change, detail string) error {
	entry := c.claude.before
	if len(entry) == 0 {
		entry = c.claude.want
	}
	return fmt.Errorf("%s; close all Claude Code sessions, check the registration, then restore in PowerShell:\n%s", detail, CommandText(claudeAddJSON(entry)))
}

// Replaceable reader lets tests schedule a real concurrent installer exactly
// between observations, without racing the filesystem or invoking a client.
var readClaudeEntry = func(path string) (json.RawMessage, error) {
	ch, err := readIntegrationChange(Scope{Global: true}, path)
	if err != nil {
		return nil, err
	}
	return claudeEntry(ch.orig)
}

func observeClaudeMCP(c Change) (string, error) {
	raw, err := readClaudeEntry(c.Path)
	if err != nil {
		return "unknown", err
	}
	if len(raw) == 0 {
		return "missing", nil
	}
	if sameClaudeEntry(raw, c.claude.want) {
		return "new", nil
	}
	if sameClaudeEntry(raw, c.claude.before) {
		return "old", nil
	}
	return "other", nil
}

func sameClaudeEntry(a, b []byte) bool {
	return bytes.Equal(a, b) || sameJSON(a, b)
}
