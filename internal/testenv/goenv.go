package testenv

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// The go command counts into os.UserConfigDir()/go/telemetry and, in a fresh
// home, may start a detached uploader that keeps writing after a fixture build
// returns, racing scratch cleanup. This is the file `go telemetry off` writes;
// only an isolated config directory is touched, never the user's real one.
func disableGoTelemetry(scratch string) error {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	if rel, err := filepath.Rel(scratch, config); err != nil || !filepath.IsLocal(rel) {
		return nil
	}
	dir := filepath.Join(config, "go", "telemetry")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "mode"), []byte("off"), 0o600)
}
