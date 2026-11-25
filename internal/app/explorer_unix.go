//go:build linux || darwin

package app

import (
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func explorerFolders() ([]string, error) {
	downloads, err := userdirs.Downloads()
	if err != nil {
		return nil, err
	}
	desktop, err := userdirs.Desktop()
	return []string{downloads, desktop}, err
}
