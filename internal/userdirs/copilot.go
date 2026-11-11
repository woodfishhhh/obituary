package userdirs

// CopilotDir follows the shared CLI/VS Code Copilot harness hook location:
// COPILOT_HOME, or ~/.copilot. Legacy VS Code profiles are not this directory.
// https://docs.github.com/en/copilot/reference/hooks-reference
func CopilotDir() (string, error) { return agentDir("COPILOT_HOME", ".copilot") }

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "copilot", Resolve: CopilotDir})
}
