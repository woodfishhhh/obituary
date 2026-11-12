//go:build linux || darwin

package winfs

import (
	"errors"
	"fmt"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

func (fs FS) ObserveWatch(path string, hold bool) (corefs.WatchSnapshot, func(), error) {
	if hold {
		return corefs.WatchSnapshot{}, nil, fmt.Errorf("excluding concurrent writers is unavailable; watch entry retained: %w", corefs.ErrUnsupported)
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ELOOP) {
		return corefs.WatchSnapshot{}, nil, corefs.ErrUnsafe
	}
	if err != nil {
		return corefs.WatchSnapshot{}, nil, err
	}
	release := func() { _ = unix.Close(fd) }
	stat, err := fs.statAt(fd, "")
	if err != nil {
		release()
		return corefs.WatchSnapshot{}, nil, err
	}
	directory := stat.Mode&unix.S_IFMT == unix.S_IFDIR
	if !directory && (stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink > 1) {
		release()
		return corefs.WatchSnapshot{}, nil, corefs.ErrUnsafe
	}
	id, err := identity(&stat, path)
	if err != nil {
		release()
		return corefs.WatchSnapshot{}, nil, err
	}
	return corefs.WatchSnapshot{Identity: id, Size: stat.Size, Modified: time.Unix(0, stat.Mtim.Nano()).UTC(), Directory: directory}, release, nil
}
