package userdirs

import "path/filepath"

// AntigravityDir is the shared App/CLI/IDE customization root, not the older
// ~/.gemini/antigravity state directory. No env override is documented.
// https://antigravity.google/docs/hooks
func AntigravityDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".gemini", "config"), err
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "antigravity", Resolve: AntigravityDir})
}
