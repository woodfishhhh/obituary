//go:build !windows

package userdirs

import (
	"os"
	"path/filepath"
)

// Autostart is the user-scoped XDG autostart directory.
func Autostart() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := Home()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	base, err := directoryAbs(base)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "autostart"), nil
}
