//go:build windows

package gui

import "path/filepath"

func companionExecutables(dir string) (string, string) {
	return filepath.Join(dir, "obit.exe"), filepath.Join(dir, "obitd.exe")
}
