package winfs

import "golang.org/x/sys/unix"

// openat2 enforces the mount boundary during lookup, including same-device bind mounts.
func openOnMount(parent int, name string, flags int) (int, error) {
	return unix.Openat2(parent, name, &unix.OpenHow{Flags: uint64(flags | unix.O_CLOEXEC | unix.O_NOFOLLOW), Resolve: unix.RESOLVE_NO_XDEV})
}
func (f FS) burialStat(parent int, name string) (posixStat, error) {
	fd, err := openOnMount(parent, name, unix.O_PATH)
	if err != nil {
		return posixStat{}, err
	}
	defer unix.Close(fd)
	return f.statAt(fd, "")
}
