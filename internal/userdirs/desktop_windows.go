package userdirs

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

// Desktop follows the redirected known folder; tests use their isolated home.
func Desktop() (string, error) {
	if os.Getenv("OBIT_USER_HOME") != "" {
		home, err := Home()
		return filepath.Join(home, "Desktop"), err
	}
	return windows.KnownFolderPath(windows.FOLDERID_Desktop, windows.KF_FLAG_DEFAULT)
}
