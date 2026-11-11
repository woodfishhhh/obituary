package userdirs

import "path/filepath"

// CodeBuddyDir follows the CLI installation/env-vars reference. The IDE uses
// the same default root; its support for the CLI override is not documented.
func CodeBuddyDir() (string, error) { return agentDir("CODEBUDDY_CONFIG_DIR", ".codebuddy") }

func CodeBuddyIDEDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".codebuddy"), err
}

// CodeBuddyLegacyMCP is runtime state, read only by the installer. Writes to
// this legacy target require the official codebuddy mcp command, not a rename.
func CodeBuddyLegacyMCP() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".codebuddy.json"), err
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "codebuddy", Resolve: CodeBuddyDir})
	RegisterAgentDirectory(AgentDirectory{Name: "codebuddy-ide", Resolve: CodeBuddyIDEDir})
}
