package userdirs

import (
	"os"
	"path/filepath"
)

// ClineDir is the current shared SDK/CLI configuration root, per
// docs.cline.bot/getting-started/config; it is independent of Roo's rules.
func ClineDir() (string, error) { return agentDir("CLINE_DIR", ".cline") }

// ClineDataDir holds settings, including cline_mcp_settings.json, separately
// from plugins and rules when CLINE_DATA_DIR overrides the default data folder.
func ClineDataDir() (string, error) {
	if dir := os.Getenv("CLINE_DATA_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	dir, err := ClineDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "cline", Resolve: ClineDir})
}
