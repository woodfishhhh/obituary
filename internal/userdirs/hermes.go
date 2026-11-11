package userdirs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// HermesDir follows hermes_constants.py::_platform_default_hermes_home and
// get_process_hermes_home. Named profiles supply their explicit HERMES_HOME.
func HermesDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("HERMES_HOME")); dir != "" {
		// Python expands user/environment syntax; require its resolved value here
		// so installation never targets a literal placeholder directory.
		if strings.HasPrefix(dir, "~") || strings.ContainsAny(dir, "$%") {
			return "", fmt.Errorf("HERMES_HOME must contain an expanded directory path")
		}
		return directoryAbs(dir)
	}
	suffix := os.Getenv("HERMES_DATA_DIR_SUFFIX")
	if strings.ContainsAny(suffix, `/\:`) || strings.HasSuffix(suffix, ".") || strings.HasSuffix(suffix, " ") {
		return "", fmt.Errorf("HERMES_DATA_DIR_SUFFIX must be a directory-name suffix")
	}
	if runtime.GOOS == "windows" {
		base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if base == "" {
			home, err := Home()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		base, err := directoryAbs(base)
		if err != nil {
			return "", err
		}
		return directoryAbs(filepath.Join(base, "hermes"+suffix))
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return directoryAbs(filepath.Join(home, ".hermes"+suffix))
}

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "hermes", Resolve: HermesDir})
}
