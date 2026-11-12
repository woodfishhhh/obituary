//go:build linux || darwin

package winfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

func singleName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\x00")
}

func (fs FS) renameNoReplace(from int, name string, to int, target string) error {
	rename := fs.rename
	if rename == nil {
		rename = renameExclusive
	}
	err := rename(from, name, to, target)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.ENOSYS) {
		return fmt.Errorf("filesystem cannot rename without replacement; nothing was overwritten (%w): %w", err, corefs.ErrUnsupported)
	}
	return err
}

// os.Root.Rename permits replacement. These platforms only pin directory
// descriptors, so verify the moved name immediately and roll back mismatches.
func (fs FS) moveInto(path string, want corefs.Identity, parent *restoreParent, name string, restore bool) (string, error) {
	real, err := fs.Resolve(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	source, err := fs.OpenRestoreParent(real)
	if err != nil {
		return "", err
	}
	defer source.Close()
	from := source.(*restoreParent)
	if parent.id.Serial != want.Serial || from.id.Serial != want.Serial {
		return "", fmt.Errorf("move parents are not on the object's device: %w", corefs.ErrUnsafe)
	}
	fd, before, err := fs.pinMoveSource(from, filepath.Base(path), want)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)
	if fs.beforeRename != nil {
		fs.beforeRename()
	}
	if err := fs.renameNoReplace(from.fd(), filepath.Base(path), parent.fd(), name); err != nil {
		return "", err
	}
	if err := fs.verifyMoved(parent.fd(), name, &before); err != nil {
		return "", fs.rollbackMove(from, filepath.Base(path), parent, name, restore, err)
	}
	return fs.verifiedChildPath(parent, name, want), nil
}

func (fs FS) pinMoveSource(parent *restoreParent, name string, want corefs.Identity) (int, posixStat, error) {
	if err := want.RequirePersistent(); err != nil {
		if _, pinned := fs.pins[want]; !pinned {
			return -1, posixStat{}, err
		}
	}
	fd, err := unix.Openat(parent.fd(), name, moveOpenFlags, 0)
	var st posixStat
	if err != nil {
		return -1, st, mapNotFound(err)
	}
	if fs.afterSourcePin != nil {
		fs.afterSourcePin()
	}
	st, err = fs.statAt(fd, "")
	if err == nil {
		err = ordinaryMoveObject(&st)
	}
	id := inodeBytes(&st)
	if err == nil && (uint64(st.Dev) != want.Serial || hex.EncodeToString(id[:]) != want.FileID || parent.id.Volume != want.Volume) {
		err = fmt.Errorf("source is no longer the tracked object; retry")
	}
	if err != nil {
		unix.Close(fd)
		return -1, st, err
	}
	return fd, st, nil
}

func ordinaryMoveObject(st *posixStat) error {
	kind := st.Mode & unix.S_IFMT
	if st.Nlink == 0 || kind != unix.S_IFDIR && (kind != unix.S_IFREG || st.Nlink != 1) {
		return fmt.Errorf("source is not an ordinary directory or singly linked file: %w", corefs.ErrUnsafe)
	}
	return nil
}

func (fs FS) verifyMoved(parent int, name string, before *posixStat) error {
	after, err := fs.statAt(parent, name)
	if err != nil {
		return err
	}
	if after.Dev != before.Dev || after.Ino != before.Ino || after.Incarnation != before.Incarnation || after.Mode&unix.S_IFMT != before.Mode&unix.S_IFMT {
		return fmt.Errorf("moved object's device, inode or type changed")
	}
	return ordinaryMoveObject(&after)
}

func (fs FS) rollbackMove(from *restoreParent, old string, to *restoreParent, name string, restore bool, cause error) error {
	if fs.beforeRollback != nil {
		fs.beforeRollback()
	}
	failure := &corefs.MoveMismatchError{Source: filepath.Join(from.path, old), Cause: cause}
	err := fs.renameNoReplace(to.fd(), name, from.fd(), old)
	if err == nil {
		return failure
	}
	failure.Rollback, failure.Retained = err, filepath.Join(to.path, name)
	if !restore {
		return failure
	}
	// A concurrently occupied grave name must not strand the restored
	// stranger outside the grave when a fresh quarantine name is available.
	target := ".identity-mismatch-" + rand.Text()
	if err := fs.renameNoReplace(to.fd(), name, from.fd(), target); err == nil {
		failure.Retained = filepath.Join(from.path, target)
	} else {
		failure.Rollback = errors.Join(failure.Rollback, err)
	}
	return failure
}

func (fs FS) verifiedChildPath(parent *restoreParent, name string, want corefs.Identity) string {
	path := filepath.Join(parent.path, name)
	id, err := fs.IdentifyFollowing(parent.path)
	if err != nil || id != parent.id {
		return ""
	}
	id, err = fs.Identify(path)
	if err != nil || id != want {
		return ""
	}
	return path
}
