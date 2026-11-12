package testenv

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Keep child builds on the caller's effective caches before HOME and XDG
// directories change. Module cache files are read-only and must not enter
// the disposable test home; go env also respects settings from GOENV.
func preserveGoEnvironment() error {
	data, err := exec.Command("go", "env", "-json", "GOMODCACHE", "GOCACHE", "GOPATH").CombinedOutput()
	if err != nil {
		return fmt.Errorf("read Go build directories: %w: %s", err, data)
	}
	var directories map[string]string
	if err := json.Unmarshal(data, &directories); err != nil {
		return err
	}
	for name, path := range directories {
		if err := os.Setenv(name, path); err != nil {
			return err
		}
	}
	return nil
}
