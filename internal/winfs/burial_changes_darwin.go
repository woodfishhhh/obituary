package winfs

import (
	"errors"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

type directoryChanges struct {
	queue int
	dirs  []int
}

func (FS) NewDirectoryChanges(root string) (corefs.DirectoryChanges, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(root, &st); err != nil {
		return nil, err
	}
	if st.Flags&unix.MNT_LOCAL == 0 {
		return nil, errors.New("cannot verify remote directory changes")
	}
	fd, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}
	unix.CloseOnExec(fd)
	return &directoryChanges{queue: fd}, nil
}
func (d *directoryChanges) WatchDirectory(path string) error {
	fd, err := unix.Open(path, unix.O_EVTONLY|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	d.dirs = append(d.dirs, fd)
	event := unix.Kevent_t{Ident: uint64(fd), Filter: unix.EVFILT_VNODE, Flags: unix.EV_ADD | unix.EV_CLEAR, Fflags: unix.NOTE_WRITE | unix.NOTE_RENAME | unix.NOTE_DELETE | unix.NOTE_REVOKE | unix.NOTE_ATTRIB}
	_, err = unix.Kevent(d.queue, []unix.Kevent_t{event}, nil, nil)
	return err
}
func (d *directoryChanges) Check() error {
	var events [1]unix.Kevent_t
	n, err := unix.Kevent(d.queue, nil, events[:], &unix.Timespec{})
	if err != nil {
		return err
	}
	if n != 0 {
		return errors.New("burial directory changed during inspection")
	}
	return nil
}
func (d *directoryChanges) Close() error {
	for _, fd := range d.dirs {
		_ = unix.Close(fd)
	}
	return unix.Close(d.queue)
}
