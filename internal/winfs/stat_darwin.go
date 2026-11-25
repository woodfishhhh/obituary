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

const hintMetadataOnly = unix.O_EVTONLY

// Darwin has no openat2(NO_XDEV). MNT_NOWAIT reads the mount table without
// contacting mounted servers; the opened descriptor's device is also checked.
func hintMountPoints() (map[string]bool, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	entries := make([]unix.Statfs_t, n+1)
	n, err = unix.Getfsstat(entries, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	if n >= len(entries) {
		return nil, unix.EAGAIN
	}
	mounts := make(map[string]bool, n)
	for i := range n {
		mounts[unix.ByteSliceToString(entries[i].Mntonname[:])] = true
	}
	return mounts, nil
}

func openHintDirectory(fd int, name string, access int) (int, error) {
	return unix.Openat(fd, name, access|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
}
