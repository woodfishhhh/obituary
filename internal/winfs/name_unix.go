//go:build linux || darwin

package winfs

import (
	"errors"
	"os"
	"path/filepath"
)

// On POSIX, differently cased basenames are distinct directory entries even
// when a hard link makes their inode identities equal.
func SameDirectoryEntry(a, b string) (bool, error) {
	if filepath.Base(a) != filepath.Base(b) {
		return false, nil
	}
	one, err := IdentifyFollowing(filepath.Dir(a))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	two, err := IdentifyFollowing(filepath.Dir(b))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil && one.Serial == two.Serial && one.FileID == two.FileID, err
}
