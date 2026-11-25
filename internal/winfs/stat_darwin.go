package winfs

import "golang.org/x/sys/unix"

func checkManagedPath(string) error { return nil }

func readPOSIXStat(fd int, name string) (posixStat, error) {
	var st unix.Stat_t
	var err error
	if name == "" {
		err = unix.Fstat(fd, &st)
	} else {
		err = unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	}
	return posixStat{Dev: uint64(st.Dev), Ino: st.Ino, Mode: uint32(st.Mode), Nlink: uint64(st.Nlink), Size: st.Size, Mtim: st.Mtim, Incarnation: birthNanos(st.Btim.Sec, st.Btim.Nsec)}, err
}

func openHintDirectory(fd int, name string) (int, error) {
	return unix.Openat(fd, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
}
