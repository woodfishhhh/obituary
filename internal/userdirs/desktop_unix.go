//go:build !windows

package userdirs

import "path/filepath"

func Desktop() (string, error) {
	home, err := Home()
	return filepath.Join(home, "Desktop"), err
}
