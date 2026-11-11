package userdirs

import "path/filepath"

// GeminiDir follows google-gemini/gemini-cli packages/core/src/utils/paths.ts
// homedir and config/storage.ts getGlobalGeminiDir: GEMINI_CLI_HOME replaces
// the HOME base, not the .gemini leaf.
func GeminiDir() (string, error) {
	home, err := agentDir("GEMINI_CLI_HOME", "")
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini"), nil
}

// GeminiSkillsDir follows the same redirected home as Gemini configuration.
func GeminiSkillsDir() (string, error) {
	home, err := agentDir("GEMINI_CLI_HOME", "")
	return filepath.Join(home, ".agents", "skills"), err
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "gemini", Resolve: GeminiDir})
}
