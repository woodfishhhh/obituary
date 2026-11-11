// Package config reads config.toml from the user's configuration directory,
// with every key optional and defaults built in. `obit config --create`
// writes a commented file with those defaults.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Delete modes.
const (
	ModeGraveyard = "graveyard" // move into Obituary's graveyard; resurrect until purged
	ModeRecycle   = "recycle"   // hand to the system Recycle Bin / Trash
)

// Config is the whole configuration file.
type Config struct {
	Graveyard         Graveyard         `toml:"graveyard"`
	Explorer          Explorer          `toml:"explorer"`
	Watch             Watch             `toml:"watch"`
	UI                UI                `toml:"ui"`
	Hooks             Hooks             `toml:"hooks"`
	CodexWindowsShell string            `toml:"codex_windows_shell,omitempty"`
	AgentShells       map[string]string `toml:"agent_shells,omitempty"`
}

// WindowsShell is the owner's declaration that this agent executes locally on
// this Windows machine using the named shell, not over SSH or in a cloud runtime.
// Codex's original root key remains an alias; Parse rejects conflicting values.
func (c Config) WindowsShell(agent string) string {
	if shell := c.AgentShells[agent]; shell != "" {
		return shell
	}
	if agent == "codex" {
		return c.CodexWindowsShell
	}
	return ""
}

// Explorer controls display-only dates in desktop.ini.
type Explorer struct {
	Enabled    bool `toml:"enabled"`
	InGit      bool `toml:"in_git"`
	Everywhere bool `toml:"everywhere"`
}

// Watch controls the quiet interval before folder-policy movement.
// Use Go duration syntax, as in time.ParseDuration.
type Watch struct {
	StableWindow string `toml:"stable_window"`
}

func (w Watch) Stability() time.Duration {
	d, _ := time.ParseDuration(w.StableWindow) // validated by Parse
	return d
}

// Hooks controls what agents are told at session start.
type Hooks struct {
	SessionSummary bool `toml:"session_summary"`
}

// UI controls native desktop and window language independently of CLI output.
type UI struct {
	Language string `toml:"language"`
}

// Graveyard controls what happens when a lifespan runs out.
type Graveyard struct {
	Mode          string `toml:"mode"`
	RetentionDays int    `toml:"retention_days"`
}

// Retention is how long burials stay restorable before they are purged.
func (g Graveyard) Retention() time.Duration {
	return time.Duration(g.RetentionDays) * 24 * time.Hour
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{Graveyard: Graveyard{Mode: ModeGraveyard, RetentionDays: 7}, Explorer: Explorer{Enabled: true}, Watch: Watch{StableWindow: "10m"}, UI: UI{Language: i18n.Auto}}
}

// Template is what `obit config --create` writes.
const Template = `# Obituary configuration. Every key is optional; these are the defaults.

# Codex native Windows shell: "bash" or "powershell".
# Unset: ambiguous Windows shell commands pass through the delete guard.
# codex_windows_shell = "powershell"

[graveyard]
# What happens when a lifespan runs out:
#   "graveyard"  move it into Obituary's graveyard (resurrect with obit resurrect)
#   "recycle"    hand it to the system Recycle Bin
mode = "graveyard"
# Days a burial stays restorable before it is purged for good.
retention_days = 7

# A declaration confirms local execution on this Windows machine in this shell.
# Values: "bash" or "powershell"; keys are agent names used by obit hook.
# Used by codex, codebuddy, codebuddy-ide and minimax-code.
# Unset agents pass ambiguous commands through; remote payloads stay skipped.
# Codex accepts either this table or codex_windows_shell above; conflicts fail.
[agent_shells]
# codex = "powershell"
# codebuddy = "bash"
# codebuddy-ide = "powershell"
# minimax-code = "powershell"

[hooks]
# At session start, also tell the agent which items in the project expire
# within 24 hours and what was buried recently. Off by default to save tokens:
# the agent is told anyway when a tool call touches a buried path.
session_summary = false

[watch]
# Quiet interval before moving a folder-policy child (Go duration syntax).
# Quiet time alone cannot prove a paused download is complete.
stable_window = "10m"

[explorer]
# Show burial dates in Explorer without changing real file names.
# By default: Downloads, Desktop (including subfolders), and folder-policy items.
enabled = true
# Label tracked items in other locations too; Git still follows in_git.
everywhere = false
# Git workspaces are excluded unless explicitly enabled here.
in_git = false

[ui]
# Native desktop and window language: "auto", "zh-CN", or "en".
# auto follows the Windows user interface language.
language = "auto"
`

// Path is the location of config.toml.
func Path() (string, error) {
	dir, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads config.toml over the defaults. A missing file is not an error.
func Load() (Config, error) {
	cfg := Default()
	path, err := Path()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return Parse(data, path)
}

// Parse decodes one configuration snapshot, also used by the installer before
// it plans a byte-preserving update through integrate.Apply.
func Parse(data []byte, path string) (Config, error) {
	cfg := Default()
	if err := toml.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &cfg); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	return cfg, cfg.validate(path)
}

func (c Config) validate(path string) error {
	if !i18n.Valid(c.UI.Language) {
		return fmt.Errorf("%s: ui.language must be %q, %q or %q", path, i18n.Auto, i18n.Chinese, i18n.English)
	}
	if c.CodexWindowsShell != "" && c.CodexWindowsShell != "bash" && c.CodexWindowsShell != "powershell" {
		return fmt.Errorf("%s: codex_windows_shell must be %q or %q, or unset", path, "bash", "powershell")
	}
	for agent, shell := range c.AgentShells {
		if shell != "bash" && shell != "powershell" {
			return fmt.Errorf("%s: agent_shells.%s must be %q or %q", path, agent, "bash", "powershell")
		}
	}
	if shell := c.AgentShells["codex"]; shell != "" && c.CodexWindowsShell != "" && shell != c.CodexWindowsShell {
		return fmt.Errorf("%s: agent_shells.codex conflicts with codex_windows_shell", path)
	}
	if d, err := time.ParseDuration(c.Watch.StableWindow); err != nil || d <= 0 {
		return fmt.Errorf("%s: watch.stable_window must be a positive duration (for example 10m)", path)
	}
	switch c.Graveyard.Mode {
	case ModeGraveyard, ModeRecycle:
	default:
		return fmt.Errorf("%s: graveyard.mode must be %q or %q, not %q", path, ModeGraveyard, ModeRecycle, c.Graveyard.Mode)
	}
	if c.Graveyard.RetentionDays < 1 || c.Graveyard.RetentionDays > 3650 {
		return fmt.Errorf("%s: graveyard.retention_days must be between 1 and 3650", path)
	}
	return nil
}

// Create writes the template unless a config file already exists.
func Create() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, []byte(Template), 0o600)
}
