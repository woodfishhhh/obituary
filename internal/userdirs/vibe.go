package userdirs

import (
	"os"
	"path/filepath"
	"strings"
)

// VibeDir follows mistralai/mistral-vibe c96a6dbb, vibe/utils/vibe_home.py:
// VIBE_HOME expands a leading ~/; otherwise ~/.vibe is the default.
func VibeDir() (string, error) {
	dir := os.Getenv("VIBE_HOME")
	if dir == "" || dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
		home, err := Home()
		if err != nil {
			return "", err
		}
		switch dir {
		case "":
			dir = filepath.Join(home, ".vibe")
		case "~":
			dir = home
		default:
			dir = home + string(filepath.Separator) + dir[2:]
		}
	}
	return directoryAbs(dir)
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "vibe", Resolve: VibeDir})
}
