//go:build linux || darwin

package app

import (
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func Guard(home string) core.Guard {
	keep := []string{home, "/", "/home", "/Users", "/Volumes", "/mnt", "/media"}
	if user, err := userdirs.Home(); err == nil {
		keep = append(keep, user)
		for _, name := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Music", "Videos", "Library", ".config", ".local"} {
			keep = append(keep, filepath.Join(user, name))
		}
	}
	return (core.Guard{Keep: keep, System: []string{home, "/bin", "/sbin", "/usr", "/etc", "/var", "/proc", "/sys", "/dev", "/boot", "/run", "/System", "/Library"}}).WithCanonical(winfs.Canonical)
}
