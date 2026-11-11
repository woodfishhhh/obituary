package userdirs

import "path/filepath"

// Only the user hook root is documented; rules/MCP must not be inferred here.
// https://docs.qoder.com/qoderwork/hooks
func QoderWorkDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".qoderwork"), err
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "qoderwork", Resolve: QoderWorkDir})
}
