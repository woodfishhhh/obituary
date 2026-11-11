package userdirs

import (
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/agentid"
)

// ZCodeDir is the standalone CLI's instruction root. At ZCode a9fe401e,
// adapters/src/config/file-config.adapter.ts loads ~/.zcode/cli/config.json.
// ZCODE_DATA_BASE_DIR redirects provider/credential storage, not this root.
func ZCodeDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zcode"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: agentid.ZCode, Resolve: ZCodeDir})
}
