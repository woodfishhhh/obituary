//go:build linux || darwin

package winfs

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

var errWatchFileOpen = errors.New("file is open")

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

// Compare kernel device/inode pairs, also for renamed files. The pinned move
// separately checks the durable identity, including generation/birth time.
func (FS) CheckWatchClosed(path string, id corefs.Identity, approved bool) error {
	if len(id.FileID) != 32 {
		return fmt.Errorf("invalid POSIX watch identity")
	}
	ino, err := strconv.ParseUint(id.FileID[16:], 16, 64)
	if err != nil {
		return err
	}
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return err
	}
	if uint64(st.Dev) != id.Serial || st.Ino != ino || st.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("watched file changed before descriptor check")
	}
	private := privateWatchFile(st.Uid, uint32(os.Geteuid()), uint32(st.Mode))
	err = checkOpenFiles(id.Serial, ino, st.Uid, private)
	return watchInventoryResult(approved, err)
}

// Continue across incomplete entries so a later confirmed holder always wins.
func recordWatchError(first *error, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errWatchFileOpen) {
		*first = err
		return true
	}
	if *first == nil {
		*first = err
	}
	return false
}

func privateWatchFile(owner, current, mode uint32) bool { return owner == current && mode&0022 == 0 }

func watchInventoryResult(approved bool, err error) error {
	if approved && !errors.Is(err, errWatchFileOpen) {
		return nil
	}
	return err
}
