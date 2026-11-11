// Package userdirs is the single place that resolves the user's directories:
// Obituary's own data and config directories and the home that agents' own
// directories (~/.claude, ~/.codex) hang off.
//
// All directory lookups go through this package. Each answer can be
// redirected through the environment, which is how tests keep away from the
// developer's real files:
//
//	OBIT_HOME        data directory (store, audit log, graveyard on that volume)
//	OBIT_CONFIG_DIR  configuration directory (config.toml)
//	OBIT_USER_HOME   the user's home, where agent configuration lives
//
// The agents' own variables (CLAUDE_CONFIG_DIR, CODEX_HOME) are honoured too.
// A test in this package scans the source to keep every lookup here.
package userdirs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

const dirName = "obituary"

// Data is where Obituary keeps its state: %LOCALAPPDATA%\obituary,
// ~/Library/Application Support/obituary or $XDG_DATA_HOME/obituary.
func Data() (string, error) {
	dir := os.Getenv("OBIT_HOME")
	if dir == "" {
		base, err := dataBase()
		if err != nil {
			return "", err
		}
		base, err = directoryAbs(base)
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, dirName)
	}
	return directoryAbs(dir)
}

// RegistryLocks is per user even when installers select different ledgers.
// TestMain redirects LOCALAPPDATA together with the other user directories.
func RegistryLocks() (string, error) {
	base, err := dataBase()
	if err != nil {
		return "", err
	}
	return directoryAbs(filepath.Join(base, dirName, "registry-locks"))
}

// Every prefix can later be used independently when a caller creates a
// directory or appends a filename. Reuse Win32 normalization via pathutil.
func directoryAbs(dir string) (string, error) {
	abs, err := pathutil.Abs(dir)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	for prefix := abs; ; prefix = pathutil.Parent(prefix) {
		if err := pathutil.Unchanged(prefix); err != nil {
			return "", err
		}
		if pathutil.Parent(prefix) == prefix {
			return abs, nil
		}
	}
}

// Config holds config.toml: %APPDATA%\obituary,
// ~/Library/Application Support/obituary or $XDG_CONFIG_HOME/obituary.
func Config() (string, error) {
	if dir := os.Getenv("OBIT_CONFIG_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	base, err = directoryAbs(base)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, dirName), nil
}

// Home is the user's home directory, which Obituary does not own.
func Home() (string, error) {
	if dir := os.Getenv("OBIT_USER_HOME"); dir != "" {
		return directoryAbs(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return directoryAbs(home)
}

func dataBase() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return dir, nil
		}
		return "", errors.New("%LOCALAPPDATA% is not set")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" && filepath.IsAbs(dir) {
			return dir, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}

// ClaudeMCPFile holds Claude's user-scoped MCP servers. With a custom
// CLAUDE_CONFIG_DIR, Claude keeps .claude.json inside that directory.
func ClaudeMCPFile() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		abs, err := directoryAbs(dir)
		return filepath.Join(abs, ".claude.json"), err
	}
	home, err := Home()
	return filepath.Join(home, ".claude.json"), err
}

// CodexSkillsDir follows the shared Agent Skills convention, independently of
// CODEX_HOME (which controls Codex configuration, not shared skills).
func CodexSkillsDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".agents", "skills"), err
}

func agentDir(env, name string) (string, error) {
	if dir := os.Getenv(env); dir != "" {
		return directoryAbs(dir)
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, name), nil
}
