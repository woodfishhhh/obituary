//go:build linux || darwin

package winfs

import (
	"fmt"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
	"io/fs"
)

// BeginImmediate keeps weak identities alive across inspection and the move.
// Closing an operation releases every inode; its weak records cannot authorize
// a later recovery or purge. Durable identities do not need retained fds.
func (fs FS) BeginImmediate() (corefs.FS, func()) {
	fs.pins = make(map[corefs.Identity]int)
	return fs, func() {
		for _, fd := range fs.pins {
			unix.Close(fd)
		}
	}
}

func (fs FS) identifyPinned(path string) (corefs.Identity, error) {
	id, _, err := fs.Inspect(path)
	return id, err
}

func (backend FS) Inspect(path string) (corefs.Identity, fs.FileMode, error) {
	fd, err := unix.Open(path, moveOpenFlags, 0)
	if err != nil {
		return corefs.Identity{}, 0, mapNotFound(err)
	}
	retained := false
	defer func() {
		if !retained {
			unix.Close(fd)
		}
	}()
	st, err := backend.statAt(fd, "")
	if err != nil {
		return corefs.Identity{}, 0, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR && st.Nlink > 1 {
		return corefs.Identity{}, 0, fmt.Errorf("multiple hard links: %w", corefs.ErrUnsafe)
	}
	id, err := identity(&st, path)
	if backend.pins != nil && err == nil && id.RequirePersistent() != nil {
		if _, exists := backend.pins[id]; !exists {
			backend.pins[id], retained = fd, true
		}
	}
	var mode fs.FileMode
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		mode = fs.ModeDir
	case unix.S_IFLNK:
		mode = fs.ModeSymlink
	}
	return id, mode, err
}
