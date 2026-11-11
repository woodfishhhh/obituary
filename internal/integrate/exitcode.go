package integrate

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed plugins/exitcode/bridge.ps1
var exitCodeScript []byte

// ExitCodeBridge only transports a PreToolUse denial. The host adapter encodes
// hook.EncodeExitCodeResponse and verifies the actual tool shell and locality.
// Native context/lifecycle hooks keep their ordinary direct obit invocation.
type ExitCodeBridge struct {
	Agent, Executable, ScriptPath string
}

func (b ExitCodeBridge) command() (string, error) {
	if !OwnedHookExec(b.Executable, []string{"hook", b.Agent}, b.Agent, nil) {
		return "", fmt.Errorf("exit-code bridge requires an absolute obit.exe path")
	}
	for _, c := range b.Agent {
		if c < 'a' || c > 'z' {
			if c != '-' {
				return "", fmt.Errorf("invalid exit-code bridge agent")
			}
		}
	}
	if !filepath.IsAbs(b.ScriptPath) || strings.ContainsAny(b.ScriptPath, "\"`$%!&|<>\x00\r\n") {
		return "", fmt.Errorf("exit-code bridge requires an absolute shell-literal script path")
	}
	return `powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "` + filepath.ToSlash(b.ScriptPath) + `"`, nil
}

// Handler returns a command hook whose timeout includes PowerShell startup and
// the bounded child transport. It requires no Node installation.
func (b ExitCodeBridge) Handler() (json.RawMessage, error) {
	command, err := b.command()
	if err != nil {
		return nil, err
	}
	return marshal(map[string]any{"type": "command", "command": command, "timeout": 10})
}

// Owns only recognises our fixed launcher and script path, without extra argv,
// alternate platform commands, async mode or shell expressions.
func (b ExitCodeBridge) Owns(raw json.RawMessage) bool {
	command, err := b.command()
	if err != nil {
		return false
	}
	entry, err := parseObject(raw)
	if err != nil {
		return false
	}
	for _, key := range entry.keys {
		if key != "command" && key != "type" && key != "timeout" {
			return false
		}
	}
	var value string
	return sameConfigJSON(entry.values["type"], []byte(`"command"`)) && json.Unmarshal(entry.values["command"], &value) == nil && value == command
}

// Plan combines a read-only registration plan with the individually receipted
// wrapper. Use HookConfig.OwnsHandler=b.Owns to register Handler(), then pass its
// changes here. Apply writes assets before registration and unregisters first.
func (b ExitCodeBridge) Plan(s Scope, registration []Change, uninstall bool) ([]Change, error) {
	if _, err := b.command(); err != nil {
		return nil, err
	}
	if len(registration) == 0 {
		return nil, fmt.Errorf("exit-code bridge requires a registration plan")
	}
	if uninstall {
		for _, ch := range registration {
			if ch.delete {
				continue
			}
			data := ch.data
			if ch.Action == "unchanged" || ch.Action == "preserve" {
				data = ch.orig
			}
			raw, err := configJSON(data, ConfigJSONC)
			if err != nil {
				return nil, fmt.Errorf("cannot verify wrapper unregistration in %s: %w", ch.Path, err)
			}
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, err
			}
			if refersToExitCodeScript(value, b.ScriptPath) {
				return nil, fmt.Errorf("%s still references %s; remove that handler explicitly before removing the wrapper", ch.Path, b.ScriptPath)
			}
		}
	}
	content := bytes.ReplaceAll(exitCodeScript, []byte("__OBIT_EXE_BASE64__"), []byte(base64.StdEncoding.EncodeToString([]byte(b.Executable))))
	content = bytes.ReplaceAll(content, []byte("__OBIT_AGENT__"), []byte(b.Agent))
	assets, err := PlanManagedAssets(s, []ManagedAsset{{Path: b.ScriptPath, ID: b.Agent + "-exitcode", Owner: b.Agent, Version: "1", Content: content}}, uninstall)
	if err != nil {
		return nil, err
	}
	return OrderManagedInstallation(assets, registration, uninstall)
}

// An edited compound handler is not ours to unregister. Do not delete the
// managed script while that retained configuration still references its path.
func refersToExitCodeScript(value any, path string) bool {
	switch v := value.(type) {
	case string:
		return strings.Contains(strings.ToLower(strings.ReplaceAll(v, `\`, "/")), strings.ToLower(filepath.ToSlash(path)))
	case []any:
		for _, item := range v {
			if refersToExitCodeScript(item, path) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if refersToExitCodeScript(item, path) {
				return true
			}
		}
	}
	return false
}
