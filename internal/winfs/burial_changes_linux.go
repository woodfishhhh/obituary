package winfs

import (
	"errors"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

type directoryChanges int

func (FS) NewDirectoryChanges(root string) (corefs.DirectoryChanges, error) {
	// Remote changes need not appear in a local inotify queue.
	var st unix.Statfs_t
	if err := unix.Statfs(root, &st); err != nil {
		return nil, err
	}
	switch st.Type {
	case unix.EXT4_SUPER_MAGIC, unix.BTRFS_SUPER_MAGIC, unix.XFS_SUPER_MAGIC, unix.TMPFS_MAGIC, unix.F2FS_SUPER_MAGIC, unix.OVERLAYFS_SUPER_MAGIC:
	default:
		return nil, errors.New("cannot verify directory changes on this filesystem")
	}
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	return directoryChanges(fd), err
}
func (d directoryChanges) WatchDirectory(path string) error {
	_, err := unix.InotifyAddWatch(int(d), path, unix.IN_ONLYDIR|unix.IN_DONT_FOLLOW|unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVE|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF|unix.IN_ATTRIB)
	return err
}

// Any queued event, including overflow or unmount, invalidates the complete inventory.
func (d directoryChanges) Check() error {
	fds := []unix.PollFd{{Fd: int32(d), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 0)
	if err != nil {
		return err
	}
	if n != 0 {
		return errors.New("burial directory changed during inspection")
	}
	return nil
}
func (d directoryChanges) Close() error { return unix.Close(int(d)) }
