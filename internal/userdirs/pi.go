package userdirs

import (
	"os"
	"path/filepath"
	"strings"
)

// PiDir is the user agent root, including extensions and settings.json.
// Source: badlogic/pi-mono packages/coding-agent/src/config.ts getAgentDir at
// 7d073328e44337d3292a7db9b0f0308f679c5e97 (including tilde expansion).
func PiDir() (string, error) {
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		if dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
			home, err := Home()
			if err != nil {
				return "", err
			}
			dir = home + dir[1:]
		}
		return directoryAbs(dir)
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pi", "agent"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "pi", Resolve: PiDir})
}
