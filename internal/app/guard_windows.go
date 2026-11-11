package app

import (
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

// knownFolders are protected wherever the user has redirected them.
var knownFolders = []*windows.KNOWNFOLDERID{
	windows.FOLDERID_Profile, windows.FOLDERID_Desktop, windows.FOLDERID_Documents, windows.FOLDERID_Downloads,
	windows.FOLDERID_Pictures, windows.FOLDERID_Videos, windows.FOLDERID_Music, windows.FOLDERID_RoamingAppData,
	windows.FOLDERID_LocalAppData, windows.FOLDERID_Public, windows.FOLDERID_SkyDrive,
}

func explorerFolders() ([]string, error) {
	var folders []string
	for _, id := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Downloads, windows.FOLDERID_Desktop} {
		path, err := windows.KnownFolderPath(id, windows.KF_FLAG_DEFAULT)
		if err != nil {
			return nil, err
		}
		folders = append(folders, path)
	}
	return folders, nil
}

// Guard returns the safety guard for the current user and data directory.
func Guard(home string) core.Guard {
	var keep, watchAllowed []string
	for _, id := range knownFolders {
		if path, err := windows.KnownFolderPath(id, windows.KF_FLAG_DEFAULT); err == nil {
			keep = append(keep, path)
			if id == windows.FOLDERID_Downloads {
				watchAllowed = append(watchAllowed, path)
			}
		}
	}
	return allowDownloads(core.DefaultGuard(home, keep...).WithCanonical(winfs.Canonical), watchAllowed)
}
