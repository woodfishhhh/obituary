package userdirs

import (
	"os"
	"path/filepath"
)

// TrashData is the freedesktop data home, independent of the Obituary ledger.
func TrashData() (string, error) {
	if base := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(base) {
		return filepath.Clean(base), nil
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}
