//go:build linux || darwin

package winfs

import "golang.org/x/sys/unix"

// Metadata supplies the inode and birth time; Linux also includes the export
// handle's generation. Neither mtime nor ctime identifies an incarnation:
// ordinary edits change both.
type posixStat struct {
	Dev, Ino, Nlink, Incarnation uint64
	Mode                         uint32
	Size                         int64
	Mtim                         unix.Timespec
}

func birthNanos(sec int64, nsec int64) uint64 {
	if sec < 0 || nsec < 0 || nsec >= 1e9 || uint64(sec) > (^uint64(0)-uint64(nsec))/1e9 {
		return 0
	}
	return uint64(sec)*1e9 + uint64(nsec)
}

func (fs FS) statAt(fd int, name string) (posixStat, error) {
	st, err := readPOSIXStat(fd, name)
	if fs.creationID != nil {
		st.Incarnation = fs.creationID(st.Incarnation)
	}
	return st, err
}

func statWithoutBirth(fd int, name string) (posixStat, error) {
	var st unix.Stat_t
	var err error
	if name == "" {
		err = unix.Fstat(fd, &st)
	} else {
		err = unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	}
	return posixStat{Dev: uint64(st.Dev), Ino: st.Ino, Mode: uint32(st.Mode), Nlink: uint64(st.Nlink), Size: st.Size, Mtim: st.Mtim}, err
}
