//go:build windows

package userdirs

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Downloads is the current user's Downloads folder, including a relocated
// one. An explicit home keeps tests inside their isolated environment.
func Downloads() (string, error) {
	if os.Getenv("OBIT_USER_HOME") != "" {
		home, err := Home()
		return filepath.Join(home, "Downloads"), err
	}
	return windows.KnownFolderPath(windows.FOLDERID_Downloads, windows.KF_FLAG_DEFAULT)
}
