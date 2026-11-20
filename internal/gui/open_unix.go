//go:build !windows

package gui

import "path/filepath"

func companionExecutables(dir string) (string, string) {
	return filepath.Join(dir, "obit"), filepath.Join(dir, "obitd")
}
