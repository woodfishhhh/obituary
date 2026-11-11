package userdirs

import "path/filepath"

// Trae uses separate international and Chinese user roots; both use .trae at
// project scope. No documented config-root environment override is assumed.
// https://docs.trae.ai/ide/hook-configuration-reference
func TraeDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".trae"), err
}

func TraeCNDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".trae-cn"), err
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "trae", Resolve: TraeDir})
	RegisterAgentDirectory(AgentDirectory{Name: "trae-cn", Resolve: TraeCNDir})
}
