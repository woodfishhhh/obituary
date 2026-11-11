package userdirs

import (
	"os"
	"path/filepath"
)

// OpenCodeDir follows anomalyco/opencode packages/util/src/global.ts and
// global-roots.ts at 894e30395c49f65fc6256902eee8f887698da7d3: an explicit
// config directory, then the XDG config root, then ~/.config/opencode.
// OPENCODE_CONFIG selects an additional config file, not the plugin directory.
func OpenCodeDir() (string, error) {
	if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		base, err := directoryAbs(base)
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "opencode"), nil
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode"), nil
}

// OpenCodeConfigOverride is the additional user-selected configuration file.
func OpenCodeConfigOverride() (string, error) {
	if path := os.Getenv("OPENCODE_CONFIG"); path != "" {
		return directoryAbs(path)
	}
	return "", nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "opencode", Resolve: OpenCodeDir})
}
