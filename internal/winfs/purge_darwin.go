package winfs

import "golang.org/x/sys/unix"

func openOnMount(parent int, name string, flags int) (int, error) {
	return unix.Openat(parent, name, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
}
func (f FS) burialStat(parent int, name string) (posixStat, error) { return f.statAt(parent, name) }
