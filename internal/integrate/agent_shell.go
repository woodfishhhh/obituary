package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanAgentWindowsShell inspects configuration without running the host
// and applies an editable declaration with the normal backup/original checks
// so shell selection preserves user edits.
func PlanAgentWindowsShell(s Scope, agent string) (Change, error) {
	if agent == "codex" {
		return PlanCodexWindowsShell(s)
	}
	path, err := config.Path()
	if err != nil {
		return Change{}, err
	}
	ch, err := readIntegrationChange(Scope{Global: true}, path)
	if err != nil {
		return ch, err
	}
	cfg, err := config.Parse(ch.orig, path)
	if err != nil {
		return ch, err
	}
	if shell := cfg.WindowsShell(agent); shell != "" {
		ch.Detail = fmt.Sprintf("preserved agent_shells.%s = %q", agent, shell)
		return ch, nil
	}
	shell, why := inferAgentWindowsShell(s, agent)
	if shell == "" {
		ch.Detail = "warning: " + why + "; " + agentShellInstructions(agent, path)
		ch.Manual, ch.Implicit = true, true // Advisory, retained separately in multi-agent plans.
		return ch, nil
	}
	data := insertAgentShell(ch.orig, agent, shell, len(cfg.AgentShells))
	if _, err := config.Parse(data, path); err != nil {
		return ch, err
	}
	ch.Detail = fmt.Sprintf("agent_shells.%s = %q inferred from %s; change this declaration if the host uses another shell or backend", agent, shell, why)
	return setIntegrationData(ch, data), nil
}

func inferAgentWindowsShell(s Scope, agent string) (string, string) {
	switch agent {
	case "codebuddy":
		return inferCodeBuddyShell(s)
	case "minimax-code":
		// minimax-code 3c21163c: local-pi-tools.ts calls getShellConfig()
		// without an override; third_party/pi-mono/.../utils/shell.ts tries
		// pwsh then powershell on Windows. Do not guess its Bash fallback.
		for _, name := range []string{"pwsh.exe", "powershell.exe"} {
			if _, err := exec.LookPath(name); err == nil {
				return "powershell", "MiniMax 3c21163c local Windows default (" + name + " on PATH)"
			}
		}
		return "", "MiniMax's local Windows PowerShell default is unavailable on PATH"
	default:
		return "", "the CodeBuddy IDE execution shell is not exposed by its documented settings"
	}
}

// CodeBuddy docs/cli/env-vars defines CODEBUDDY_CODE_SHELL, while the
// Git Bash path configures the hook launcher and cannot attest a tool shell.
// settings.json env entries are inspected too; conflicting launch contexts
// require a manual declaration instead of guessing which client will run.
func inferCodeBuddyShell(s Scope) (string, string) {
	dir, err := userdirs.CodeBuddyDir()
	if err != nil {
		return "", err.Error()
	}
	paths := []string{filepath.Join(dir, "settings.json")}
	if !s.Global {
		paths = append(paths, filepath.Join(s.Project, ".codebuddy", "settings.json"), filepath.Join(s.Project, ".codebuddy", "settings.local.json"))
	}
	values := []string{os.Getenv("CODEBUDDY_CODE_SHELL")}
	for _, key := range []string{"CODEBUDDY_CODE_SHELL_PREFIX", "CODEBUDDY_SANDBOX_IMAGE"} {
		if os.Getenv(key) != "" {
			return "", key + " may select a wrapper or nonlocal backend"
		}
	}
	for _, path := range paths {
		value, err := codeBuddyConfiguredShell(path)
		if err != nil {
			return "", err.Error()
		}
		values = append(values, value)
	}
	shell := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if value != "bash" && value != "powershell" || shell != "" && shell != value {
			return "", "CODEBUDDY_CODE_SHELL is unsupported or conflicts across launch settings"
		}
		shell = value
	}
	if shell == "" {
		return "", "CODEBUDDY_CODE_SHELL is not configured"
	}
	return shell, "CODEBUDDY_CODE_SHELL in the current environment or host settings"
}

func codeBuddyConfiguredShell(path string) (string, error) {
	ch, err := readIntegrationChange(Scope{Global: true}, path)
	if err != nil {
		return "", err
	}
	raw, err := configJSON(ch.orig, ConfigJSONC)
	if err != nil {
		return "", fmt.Errorf("cannot parse CodeBuddy settings %s: %w", path, err)
	}
	var settings struct {
		Env     map[string]string      `json:"env"`
		Sandbox struct{ Enabled bool } `json:"sandbox"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return "", fmt.Errorf("cannot read CodeBuddy shell settings %s: %w", path, err)
	}
	if settings.Sandbox.Enabled || settings.Env["CODEBUDDY_CODE_SHELL_PREFIX"] != "" || settings.Env["CODEBUDDY_SANDBOX_IMAGE"] != "" {
		return "", fmt.Errorf("CodeBuddy settings %s select a sandbox or shell wrapper", path)
	}
	return settings.Env["CODEBUDDY_CODE_SHELL"], nil
}

// Use go-toml's AST offsets, as RefreshTOMLString does, to insert one member
// without reserializing user comments, inline tables, root keys or the BOM.
func insertAgentShell(original []byte, agent, shell string, members int) []byte {
	bom := len(original) - len(bytes.TrimPrefix(original, []byte("\xef\xbb\xbf")))
	data := original[bom:]
	position, insertion := 0, fmt.Sprintf("agent_shells.%s = %q\n", agent, shell)
	p := unstable.Parser{}
	p.Reset(data)
	root := true
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind == unstable.Table || n.Kind == unstable.ArrayTable {
			root = false
		}
		keys := n.Key()
		if !keys.Next() || string(keys.Node().Data) != "agent_shells" {
			continue
		}
		keyEnd := int(keys.Node().Raw.Offset + keys.Node().Raw.Length)
		if keys.Next() {
			continue
		}
		if n.Kind == unstable.Table {
			position = keyEnd + bytes.IndexByte(data[keyEnd:], '\n') + 1
			if position == keyEnd {
				position = len(data)
			}
			insertion = fmt.Sprintf("\n%s = %q\n", agent, shell)
			break
		}
		if root && n.Kind == unstable.KeyValue && n.Value().Kind == unstable.InlineTable {
			position = int(n.Value().Raw.Offset) + 1
			insertion = fmt.Sprintf("%s = %q", agent, shell)
			if members > 0 {
				insertion += ", "
			}
			break
		}
	}
	position += bom
	return append(append(bytes.Clone(original[:position]), insertion...), original[position:]...)
}

func agentShellInstructions(agent, path string) string {
	return fmt.Sprintf("in %s, add or update %s = \"bash\" or \"powershell\" under [agent_shells] after confirming local Windows execution; undeclared commands pass through", path, agent)
}

// AgentWindowsShellStatus reports the declaration actually read by hooks.
func AgentWindowsShellStatus(agent string) (detail string, warning bool, err error) {
	if agent != "codebuddy" && agent != "codebuddy-ide" && agent != "minimax-code" {
		return "", false, nil
	}
	path, err := config.Path()
	if err != nil {
		return "", false, err
	}
	cfg, err := config.Load()
	if err != nil {
		return "", false, err
	}
	if shell := cfg.WindowsShell(agent); shell != "" {
		return fmt.Sprintf("agent_shells.%s = %q; declared local Windows shell; remote payloads pass through", agent, shell), false, nil
	}
	return "agent_shells." + agent + " is unset; " + agentShellInstructions(agent, path), true, nil
}
