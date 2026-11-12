//go:build linux || darwin

package winfs

import (
	"fmt"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
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
	fd, err := unix.Open(path, moveOpenFlags, 0)
	if err != nil {
		return corefs.Identity{}, mapNotFound(err)
	}
	retained := false
	defer func() {
		if !retained {
			unix.Close(fd)
		}
	}()
	st, err := fs.statAt(fd, "")
	if err != nil {
		return corefs.Identity{}, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR && st.Nlink > 1 {
		return corefs.Identity{}, fmt.Errorf("multiple hard links: %w", corefs.ErrUnsafe)
	}
	id, err := identity(&st, path)
	if err == nil && id.RequirePersistent() != nil {
		if _, exists := fs.pins[id]; !exists {
			fs.pins[id], retained = fd, true
		}
	}
	return id, err
}
