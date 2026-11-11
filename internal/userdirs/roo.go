package userdirs

// RooDir is the global rules root documented by Roo Code custom instructions.
func RooDir() (string, error) { return agentDir("", ".roo") }

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "roo", Resolve: RooDir})
}
