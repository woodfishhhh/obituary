package integrate

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanCodexWindowsShell stores an owner's editable declaration in Obituary's
// config, using the same original-check/backup/atomic-write plan as hook setup.
// Inspection only reads Codex configuration, leaving client settings and trust
// intact. Uninstall preserves this user setting.
func PlanCodexWindowsShell(s Scope) (Change, error) {
	path, err := config.Path()
	if err != nil {
		return Change{}, err
	}
	ch, err := readIntegrationChange(Scope{Global: true, inspection: s.inspection}, path)
	if err != nil {
		return ch, err
	}
	cfg, err := config.Parse(ch.orig, path)
	if err != nil {
		return ch, err
	}
	if shell := cfg.AgentShells["codex"]; shell != "" {
		ch.Detail = fmt.Sprintf("preserved agent_shells.codex = %q", shell)
		return ch, nil
	}
	var keys map[string]any
	if err := toml.Unmarshal(bytes.TrimPrefix(ch.orig, []byte("\xef\xbb\xbf")), &keys); err != nil {
		return ch, err
	}
	if _, exists := keys["codex_windows_shell"]; exists {
		ch.Detail = "preserved codex_windows_shell = " + fmt.Sprintf("%q", cfg.CodexWindowsShell)
		return ch, nil
	}
	why := inferCodexWindowsShell(s)
	if why != "" {
		ch.Detail = "warning: " + why + "; set codex_windows_shell = \"bash\" or \"powershell\" manually in this file; undeclared Windows shell commands pass through"
		return ch, nil
	}
	// Prefix a root key instead of appending after an arbitrary TOML table.
	// go-toml parses the existing file; no hand-written TOML editing is needed.
	bom := []byte("\xef\xbb\xbf")
	data := []byte("codex_windows_shell = \"powershell\"\n")
	data = append(data, bytes.TrimPrefix(ch.orig, bom)...)
	if bytes.HasPrefix(ch.orig, bom) {
		data = append(bytes.Clone(bom), data...)
	}
	ch.Detail = "codex_windows_shell = \"powershell\" inferred from Codex 0.167 native Windows default; change this declaration if your client overrides the shell (per-call shell is absent from hooks)"
	return setIntegrationData(ch, data), nil
}

// Codex rust-v0.167.0 shell-command/src/shell_detect.rs:
// default_user_shell_from_path chooses PowerShell on Windows (pwsh, then
// powershell), with cmd as the final fallback. config/src/config_toml.rs and
// types.rs expose no shell-selection key; shell_environment_policy only sets
// the child environment. exec_command.shell can override per call but is lost
// in its PreToolUse payload. Opaque desktop settings cannot be inferred here.
// https://github.com/openai/codex/blob/rust-v0.167.0/codex-rs/shell-command/src/shell_detect.rs
func inferCodexWindowsShell(s Scope) string {
	dir, err := userdirs.CodexDir()
	if err != nil {
		return err.Error()
	}
	paths := []string{filepath.Join(dir, "config.toml")}
	if !s.Global {
		paths = append(paths, filepath.Join(s.Project, ".codex", "config.toml"))
	}
	for _, path := range paths {
		ch, err := readIntegrationChange(Scope{Global: true}, path)
		if err != nil {
			return "cannot read Codex config: " + err.Error()
		}
		var cfg map[string]any
		if err := toml.Unmarshal(bytes.TrimPrefix(ch.orig, []byte("\xef\xbb\xbf")), &cfg); err != nil {
			return "cannot parse Codex config " + path + ": " + err.Error()
		}
		if desktop, ok := cfg["desktop"].(map[string]any); ok && len(desktop) > 0 {
			return "Codex desktop shell cannot be inferred from opaque desktop settings in " + path
		}
	}
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		if _, err := exec.LookPath(name); err == nil {
			return ""
		}
	}
	return "Codex's PowerShell default cannot be verified on PATH (its cmd fallback is unsupported)"
}
