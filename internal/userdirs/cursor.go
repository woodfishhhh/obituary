package userdirs

import "path/filepath"

// CursorDir is the user hooks/MCP root documented at cursor.com/docs/hooks.
// No official environment override for this location is documented.
func CursorDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cursor"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "cursor", Resolve: CursorDir})
}
