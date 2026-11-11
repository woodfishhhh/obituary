package userdirs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// DeepSeekDir follows dsh-home-paths at fcc0ad46: blank overrides fall back,
// and only the current user's ~, ~/ and ~\ forms expand.
func DeepSeekDir() (string, error) {
	value := os.Getenv("DSH_HOME")
	if strings.TrimSpace(value) == "" {
		value = ""
	}
	if value == "" || value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := Home()
		if err != nil {
			return "", err
		}
		switch value {
		case "":
			value = filepath.Join(home, ".dsh")
		case "~":
			value = home
		default:
			value = filepath.Join(home, value[2:])
		}
	}
	return directoryAbs(value)
}

// DeepSeekProfileDir resolves an explicit profile; it never chooses a user's
// active profile or writes the package-manager-owned profile manifest.
func DeepSeekProfileDir(name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`) {
		return "", fmt.Errorf("DeepSeek profile must be one directory name")
	}
	root, err := DeepSeekDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "profiles", name)
	if err := pathutil.Unchanged(path); err != nil {
		return "", err
	}
	return directoryAbs(path)
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "deepseek-harness", Resolve: DeepSeekDir})
}
