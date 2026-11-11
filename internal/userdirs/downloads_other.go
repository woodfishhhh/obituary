//go:build !windows

package userdirs

import "path/filepath"

// Downloads is ~/Downloads, the common default outside Windows.
func Downloads() (string, error) {
	home, err := Home()
	return filepath.Join(home, "Downloads"), err
}
