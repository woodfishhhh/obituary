package userdirs

import "path/filepath"

// Lingma retains .lingma even after the product rename. No root override is
// documented for this IDE/plugin: docs.qoder.cn/user-guide/hooks.
func LingmaDir() (string, error) {
	home, err := Home()
	return filepath.Join(home, ".lingma"), err
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "lingma", Resolve: LingmaDir})
}
