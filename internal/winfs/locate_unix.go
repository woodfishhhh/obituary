//go:build linux || darwin

package winfs

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

// LocateIn searches only a known buried tree. os.Root prevents concurrent
// ancestor symlinks from redirecting enumeration outside that tree. Absence
// here is uncertainty about the inode's location, never permission to drop it.
func (f FS) LocateIn(dir string, want corefs.Identity) (string, error) {
	parent, err := f.Identify(dir)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	found := ""
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if matchesPOSIXInfo(info, want) {
			found = filepath.Join(dir, name)
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("contained inode cannot be located in %s; retain recovery evidence: %w", dir, corefs.ErrUnsupported)
	}
	if got, err := f.Identify(dir); err != nil || got != parent {
		return "", fmt.Errorf("buried parent changed during child lookup; retry")
	}
	if got, err := f.Identify(found); err != nil || got != want {
		return "", fmt.Errorf("buried child changed during lookup; retry")
	}
	return found, nil
}

func matchesPOSIXInfo(info os.FileInfo, want corefs.Identity) bool {
	st := info.Sys().(*syscall.Stat_t)
	stat := unix.Stat_t{Dev: st.Dev, Ino: st.Ino, Mode: st.Mode, Nlink: st.Nlink}
	if ordinaryMoveObject(&stat) != nil {
		return false
	}
	id := inodeBytes(&stat)
	return uint64(st.Dev) == want.Serial && hex.EncodeToString(id[:]) == want.FileID
}
