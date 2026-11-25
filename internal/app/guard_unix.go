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
	var watchAllowed []string
	if user, err := userdirs.Home(); err == nil {
		watchAllowed = []string{filepath.Join(user, "Downloads")}
		keep = append(keep, user)
		for _, name := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Music", "Videos", "Library", ".config", ".local"} {
			keep = append(keep, filepath.Join(user, name))
		}
	}
	g := (core.Guard{Keep: keep, System: []string{home, "/bin", "/sbin", "/usr", "/etc", "/var", "/proc", "/sys", "/dev", "/boot", "/run", "/System", "/Library"}, Exempt: []string{ReportsDir(home)}}).WithCanonical(winfs.ManagedPath)
	return allowDownloads(g, watchAllowed)
}
