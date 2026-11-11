//go:build windows

package userdirs

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Programs is the current user's redirected Start menu Programs folder.
// Explicit homes keep diagnostics inside the same isolated environment as init.
func Programs() (string, error) {
	if dir := os.Getenv("OBIT_START_MENU_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	if os.Getenv("OBIT_USER_HOME") != "" {
		home, err := Home()
		return filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs"), err
	}
	return windows.KnownFolderPath(windows.FOLDERID_Programs, windows.KF_FLAG_DEFAULT)
}
