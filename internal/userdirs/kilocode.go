package userdirs

import (
	"os"
	"path/filepath"
)

// KiloCodeDir follows Kilo-Org/kilocode packages/core/src/global.ts and
// flag/flag.ts: KILO_CONFIG_DIR, then XDG_CONFIG_HOME/kilo, then ~/.config/kilo.
// This is the current plugin-capable client; legacy .kilocode rules are separate.
func KiloCodeDir() (string, error) {
	if dir := os.Getenv("KILO_CONFIG_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := Home()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	base, err := directoryAbs(base)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "kilo"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "kilocode", Resolve: KiloCodeDir})
}
