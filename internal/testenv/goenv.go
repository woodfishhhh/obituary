package testenv

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// The go command counts into <user config dir>/go/telemetry and, in a fresh
// home, may start a detached uploader that keeps writing after a fixture build
// returns, racing scratch cleanup. This is the file `go telemetry off` writes,
// in the config directory isolateDirectories gives HOME / XDG_CONFIG_HOME
// (TestGoTelemetryOffInIsolatedConfig catches drift). Windows keeps the
// user's own %AppData%, so it is left alone.
func disableGoTelemetry(scratch string) error {
	var config string
	switch runtime.GOOS {
	case "windows":
		return nil
	case "darwin":
		config = filepath.Join(scratch, "home", "Library", "Application Support")
	default:
		config = filepath.Join(scratch, "xdg-config")
	}
	dir := filepath.Join(config, "go", "telemetry")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "mode"), []byte("off"), 0o600)
}
