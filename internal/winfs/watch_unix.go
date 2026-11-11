//go:build linux || darwin

package winfs

import (
	"errors"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/unix"
)

func (FS) ObserveWatch(path string, hold bool) (core.WatchSnapshot, func(), error) {
	if hold {
		return core.WatchSnapshot{}, nil, unsupportedMove()
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ELOOP) {
		return core.WatchSnapshot{}, nil, core.ErrUnsafe
	}
	if err != nil {
		return core.WatchSnapshot{}, nil, err
	}
	release := func() { _ = unix.Close(fd) }
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		release()
		return core.WatchSnapshot{}, nil, err
	}
	directory := stat.Mode&unix.S_IFMT == unix.S_IFDIR
	if !directory && (stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink > 1) {
		release()
		return core.WatchSnapshot{}, nil, core.ErrUnsafe
	}
	id, err := identity(&stat, path)
	if err != nil {
		release()
		return core.WatchSnapshot{}, nil, err
	}
	return core.WatchSnapshot{Identity: id, Size: stat.Size, Modified: time.Unix(0, stat.Mtim.Nano()).UTC(), Directory: directory}, release, nil
}
