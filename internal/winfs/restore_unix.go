//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

type restoreParent struct {
	file    *os.File
	path    string
	id      corefs.Identity
	created bool
}

func (p *restoreParent) Close() error              { return p.file.Close() }
func (p *restoreParent) Path() string              { return p.path }
func (p *restoreParent) Identity() corefs.Identity { return p.id }
func (p *restoreParent) fd() int                   { return int(p.file.Fd()) }
func (p *restoreParent) Rollback() error {
	if p.created {
		// POSIX cannot unlink a directory by its creation fd. A pathname
		// recheck followed by unlinkat could remove somebody else's directory.
		return fmt.Errorf("new restore parent retained at %s; inspect and remove it manually if empty", p.path)
	}
	return nil
}

func (FS) OpenRestoreParent(path string) (corefs.RestoreParent, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		f.Close()
		return nil, err
	}
	id, err := identity(&st, path)
	if err != nil {
		f.Close()
		return nil, err
	}
	real, err := Canonical(path)
	if err != nil {
		f.Close()
		return nil, err
	}
	back, err := (FS{}).Identify(real)
	if err != nil || back != id {
		f.Close()
		return nil, fmt.Errorf("restore parent changed while opening %s: %w", path, corefs.ErrUnsafe)
	}
	return &restoreParent{file: f, path: real, id: id}, nil
}

func (FS) MkdirRestoreParent(parent corefs.RestoreParent, name string) (corefs.RestoreParent, error) {
	p, ok := parent.(*restoreParent)
	if !ok || !singleName(name) {
		return nil, fmt.Errorf("invalid restore parent or filename: %w", corefs.ErrUnsafe)
	}
	err := unix.Mkdirat(p.fd(), name, 0700)
	created := err == nil
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	fd, err := unix.Openat(p.fd(), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(p.path, name)
	f := os.NewFile(uintptr(fd), path)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		f.Close()
		return nil, err
	}
	id, err := identity(&st, path)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &restoreParent{file: f, path: path, id: id, created: created}, nil
}

// FindRestoredIn supplies a spelling, never an identity assertion. The pinned
// parent and each candidate must still match the durable restore intent.
func (fs FS) FindRestoredIn(path string, parent, source corefs.Identity) (string, error) {
	real, err := fs.Resolve(path)
	if err != nil {
		return "", fmt.Errorf("cannot locate recorded restore parent: %w", err)
	}
	opened, err := fs.OpenRestoreParent(real)
	if err != nil {
		return "", err
	}
	defer opened.Close()
	p := opened.(*restoreParent)
	if p.id != parent {
		return "", fmt.Errorf("recorded restore parent is no longer at %s; identity lookup is unavailable; retain record", path)
	}
	return findRestoredChild(p, source)
}

func (fs FS) FindRestored(parent, source corefs.Identity) (string, error) {
	return fs.FindRestoredIn(parent.Volume, parent, source)
}

func findRestoredChild(parent *restoreParent, source corefs.Identity) (string, error) {
	entries, err := parent.file.ReadDir(-1)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		fd, _, err := pinMoveSource(parent, entry.Name(), source)
		if err != nil {
			continue
		}
		unix.Close(fd)
		if path := verifiedChildPath(parent, entry.Name(), source); path != "" {
			return path, nil
		}
	}
	// A failed candidate inspection cannot prove absence of the inode.
	return "", fmt.Errorf("restored object could not be verified in its recorded parent; retain record")
}
