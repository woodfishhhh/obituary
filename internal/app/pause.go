package app

import (
	"errors"
	"os"
	"path/filepath"
)

// Paused is shared by the tray and the scheduled --once fallback.
func Paused(home string) (bool, error) {
	_, err := os.Stat(filepath.Join(home, "obitd.paused"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// SetPaused affects future automatic ticks; an in-flight commit finishes.
func SetPaused(home string, paused bool) error {
	path := filepath.Join(home, "obitd.paused")
	if paused {
		if err := os.MkdirAll(home, 0o700); err != nil {
			return err
		}
		return os.WriteFile(path, nil, 0o600)
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
