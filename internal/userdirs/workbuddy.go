package userdirs

import "path/filepath"

// WorkBuddyDir is the mainland Windows desktop configuration root. The official
// MCP guide names ~/.workbuddy; the international edition needs separate path
// verification. No documented configuration-root environment override is assumed.
// https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/MCP-Guide
func WorkBuddyDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".workbuddy"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "workbuddy", Resolve: WorkBuddyDir})
}
