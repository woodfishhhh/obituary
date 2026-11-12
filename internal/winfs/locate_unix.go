//go:build linux || darwin

package winfs

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

// LocateIn searches only a known buried tree. os.Root prevents concurrent
// ancestor symlinks from redirecting enumeration outside that tree. Absence
// here is uncertainty about the inode's location, never permission to drop it.
func (f FS) LocateIn(dir string, want corefs.Identity) (string, error) {
	if err := want.RequirePersistent(); err != nil {
		return "", err
	}
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
		matches, err := f.matchesPOSIXInfo(root, name, want)
		if err != nil {
			return err
		}
		if matches {
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

func (f FS) matchesPOSIXInfo(root *os.Root, name string, want corefs.Identity) (bool, error) {
	file, err := root.OpenFile(name, moveOpenFlags, 0)
	if err != nil {
		return false, err
	}
	defer file.Close()
	st, err := f.statAt(int(file.Fd()), "")
	if err != nil {
		return false, err
	}
	if ordinaryMoveObject(&st) != nil {
		return false, nil
	}
	id := inodeBytes(&st)
	return st.Dev == want.Serial && hex.EncodeToString(id[:]) == want.FileID, nil
}
