//go:build linux || darwin

package pathutil

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// MountRoot follows existing ancestors and stops where st_dev changes. Missing
// suffixes inherit their nearest existing parent's mount; access errors do not.
func MountRoot(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var st unix.Stat_t
	for {
		err = unix.Lstat(abs, &st)
		if !errors.Is(err, unix.ENOENT) {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", err
		}
		abs = parent
	}
	if err != nil {
		return "", err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		abs = filepath.Dir(abs)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	if err := unix.Stat(abs, &st); err != nil {
		return "", err
	}
	for {
		parent := filepath.Dir(abs)
		if parent == abs {
			return abs, nil
		}
		var up unix.Stat_t
		if err := unix.Stat(parent, &up); err != nil {
			return "", err
		}
		if up.Dev != st.Dev {
			return abs, nil
		}
		abs = parent
	}
}

// VolumeRoot is also used for lexical parent traversal. Identity operations
// call MountRoot directly so an unavailable mount remains an explicit error.
func VolumeRoot(path string) string {
	root, err := MountRoot(path)
	if err != nil {
		return "/"
	}
	return root
}
