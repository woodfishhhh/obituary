package userdirs

import (
	"os"
	"path/filepath"
)

// SystemdUser is always user-scoped; it never resolves /etc/systemd/system.
func SystemdUser() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := Home()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	abs, err := directoryAbs(base)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, "systemd", "user"), nil
}
