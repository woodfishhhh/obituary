//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/unix"
)

type restoreParent struct {
	file *os.File
	path string
	id   core.Identity
}

func (p *restoreParent) Close() error            { return p.file.Close() }
func (p *restoreParent) Path() string            { return p.path }
func (p *restoreParent) Identity() core.Identity { return p.id }
func (p *restoreParent) Rollback() error         { return nil } // only existing parents are opened

func (FS) OpenRestoreParent(path string) (core.RestoreParent, error) {
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
		return nil, fmt.Errorf("restore parent changed while opening %s: %w", path, core.ErrUnsafe)
	}
	return &restoreParent{file: f, path: real, id: id}, nil
}

// mkdirat cannot provide rollback-by-creation-handle. Reject before creating
// anything until POSIX restoration has an explicitly approved contract.
func (FS) MkdirRestoreParent(core.RestoreParent, string) (core.RestoreParent, error) {
	return nil, unsupportedMove()
}
func (FS) FindRestored(core.Identity, core.Identity) (string, error) { return "", unsupportedMove() }
