//go:build linux || darwin

package winfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

// RemoveAll has no mount boundary. Open each directory relative to the pinned parent,
// then check its device on that same handle before enumerating or unlinking children.
func (f FS) PurgeGrave(path string, want corefs.Identity, protected corefs.ProtectedIDs) error {
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	return f.removeGraveDir(int(parent.Fd()), filepath.Base(path), want, -1, protected)
}
func (f FS) removeGraveDir(parent int, name string, want corefs.Identity, stage int, protected corefs.ProtectedIDs) error {
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
	if st.Dev != want.Serial || stage < 0 && hex.EncodeToString(id[:]) != want.FileID || len(protected) > 0 && (st.Incarnation == 0 || protected[hex.EncodeToString(id[:])]) {
		return corefs.ErrUnsafe
	}
	root := stage < 0
	if root {
		stage = fd
	}
	for {
		names, err := dir.Readdirnames(64)
		for _, child := range names {
			if err := f.removeGraveEntry(fd, child, want, stage, protected); err != nil {
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
	if root {
		return unix.Unlinkat(parent, name, unix.AT_REMOVEDIR)
	}
	return f.unlinkGraveEntry(parent, name, unix.AT_REMOVEDIR, st, stage, protected)
}
func (f FS) removeGraveEntry(parent int, name string, want corefs.Identity, stage int, protected corefs.ProtectedIDs) error {
	st, err := f.burialStat(parent, name)
	if err != nil {
		return err
	}
	if st.Dev != want.Serial {
		return corefs.ErrUnsafe
	}
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		return f.removeGraveDir(parent, name, want, stage, protected)
	}
	return f.unlinkGraveEntry(parent, name, 0, st, stage, protected)
}

// unlinkat selects a name, not a file descriptor. With active registrations,
// take the name into a fresh private directory, then verify the object there.
func (f FS) unlinkGraveEntry(parent int, name string, flags int, expected posixStat, stage int, protected corefs.ProtectedIDs) error {
	if len(protected) == 0 {
		return unix.Unlinkat(parent, name, flags)
	}
	id := inodeBytes(&expected)
	if expected.Incarnation == 0 || protected[hex.EncodeToString(id[:])] {
		return fmt.Errorf("protected active object %s: %w", name, corefs.ErrUnsafe)
	}
	if parent == stage {
		return unix.Unlinkat(parent, name, flags)
	}
	scratch := ".obit-purge-" + rand.Text()
	if err := unix.Mkdirat(stage, scratch, 0700); err != nil {
		return err
	}
	fd, err := openOnMount(stage, scratch, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	if err = renameExclusive(parent, name, fd, name); err != nil {
		return errors.Join(err, unix.Unlinkat(stage, scratch, unix.AT_REMOVEDIR))
	}
	err = f.deleteStagedEntry(fd, name, flags, expected, protected)
	if err != nil {
		if rollback := renameExclusive(fd, name, parent, name); rollback != nil {
			return fmt.Errorf("retained %s/%s: %w", scratch, name, errors.Join(err, rollback))
		}
	}
	return errors.Join(err, unix.Unlinkat(stage, scratch, unix.AT_REMOVEDIR))
}
func (f FS) deleteStagedEntry(parent int, name string, flags int, want posixStat, protected corefs.ProtectedIDs) error {
	fd, err := openOnMount(parent, name, moveOpenFlags)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	got, err := f.statAt(fd, "")
	if err != nil {
		return err
	}
	id := inodeBytes(&got)
	if got.Dev != want.Dev || id != inodeBytes(&want) || protected[hex.EncodeToString(id[:])] {
		return fmt.Errorf("changed or protected entry %s: %w", name, corefs.ErrUnsafe)
	}
	return unix.Unlinkat(parent, name, flags)
}
