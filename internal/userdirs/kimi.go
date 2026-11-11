package userdirs

import "path/filepath"

// KimiDir targets kimi-cli's Beta hook configuration. See MoonshotAI/kimi-cli
// 9ab1286b src/kimi_cli/share.py. The newer kimi-code product uses another root.
func KimiDir() (string, error) { return agentDir("KIMI_SHARE_DIR", ".kimi") }

// KimiSkillsDir is independent of KIMI_SHARE_DIR, per the official skills docs.
func KimiSkillsDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".kimi", "skills"), err
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "kimi", Resolve: KimiDir})
}
