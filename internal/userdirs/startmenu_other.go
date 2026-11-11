//go:build !windows

package userdirs

import (
	"errors"
	"os"
)

// Programs has no Start menu equivalent outside Windows; an explicit
// OBIT_START_MENU_DIR keeps tests and diagnostics portable.
func Programs() (string, error) {
	if dir := os.Getenv("OBIT_START_MENU_DIR"); dir != "" {
		return directoryAbs(dir)
	}
	return "", errors.New("the Start menu is Windows-only")
}
