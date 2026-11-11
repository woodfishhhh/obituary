package userdirs

import "path/filepath"

// DroidDir is ~/.factory (docs.factory.com/harness/hooks).
// FACTORY_HOME_OVERRIDE names the containing home; append .factory to obtain
// the host's configuration directory.
func DroidDir() (string, error) {
	home, err := agentDir("FACTORY_HOME_OVERRIDE", "")
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".factory"), nil
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "droid", Resolve: DroidDir})
}
