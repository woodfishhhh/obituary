package userdirs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OpenClawDir is the native Gateway's state and extension root. Config file
// overrides are independent; use OpenClawConfigFile for openclaw.json.
// Sources: openclaw/openclaw src/config/{state-dir,paths}.ts and
// src/infra/home-dir.ts at 3acbd8cbd5f76b81b9ef2637b19d71c4cb4c47f8.
func OpenClawDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("OPENCLAW_STATE_DIR")); dir != "" {
		return openClawPath(dir)
	}
	profile := strings.TrimSpace(os.Getenv("OPENCLAW_PROFILE"))
	if profile != "" && !strings.EqualFold(profile, "default") {
		return "", fmt.Errorf("OpenClaw named profiles require an explicit OPENCLAW_STATE_DIR")
	}
	home, err := openClawHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openclaw"), nil
}

// OpenClawConfigFile resolves a JSON/JSON5 configuration path without reading it.
func OpenClawConfigFile() (string, error) {
	if path := strings.TrimSpace(os.Getenv("OPENCLAW_CONFIG_PATH")); path != "" {
		return openClawPath(path)
	}
	dir, err := OpenClawDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "openclaw.json"), nil
}

func openClawHome() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("OPENCLAW_HOME")); dir != "" && dir != "undefined" && dir != "null" {
		if dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
			home, err := Home()
			if err != nil {
				return "", err
			}
			dir = home + dir[1:]
		}
		return directoryAbs(dir)
	}
	return Home()
}

func openClawPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := openClawHome()
		if err != nil {
			return "", err
		}
		path = home + path[1:]
	}
	return directoryAbs(path)
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "openclaw", Resolve: OpenClawDir})
}
