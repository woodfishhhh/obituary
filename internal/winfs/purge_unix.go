//go:build linux || darwin

package winfs

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

// RemoveAll has no mount boundary. Open each directory relative to the pinned parent,
// then check its device on that same handle before enumerating or unlinking children.
func (f FS) PurgeGrave(path string, want corefs.Identity) error {
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	return f.removeGraveDir(int(parent.Fd()), filepath.Base(path), want, true)
}
func (f FS) removeGraveDir(parent int, name string, want corefs.Identity, root bool) error {
	fd, err := openOnMount(parent, name, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(fd), name)
	defer dir.Close()
	st, err := f.statAt(fd, "")
	if err != nil {
		return err
	}
	id := inodeBytes(&st)
	if st.Dev != want.Serial || root && hex.EncodeToString(id[:]) != want.FileID {
		return corefs.ErrUnsafe
	}
	for {
		names, err := dir.Readdirnames(64)
		for _, child := range names {
			if err := f.removeGraveEntry(fd, child, want); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	return unix.Unlinkat(parent, name, unix.AT_REMOVEDIR)
}
func (f FS) removeGraveEntry(parent int, name string, want corefs.Identity) error {
	st, err := f.burialStat(parent, name)
	if err != nil {
		return err
	}
	if st.Dev != want.Serial {
		return corefs.ErrUnsafe
	}
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		return f.removeGraveDir(parent, name, want, false)
	}
	return unix.Unlinkat(parent, name, 0)
}
