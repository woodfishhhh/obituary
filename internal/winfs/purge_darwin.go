package winfs

import (
	"errors"

	"golang.org/x/sys/unix"
)

func openOnMount(parent int, name string, flags int) (int, error) {
	return unix.Openat(parent, name, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
}
func (f FS) burialStat(parent int, name string) (posixStat, error) { return f.statAt(parent, name) }

func (p *purgeTree) tryUnlinkWithoutOpen(fd int, name string, want posixStat, openErr error) (bool, error) {
	mode := want.Mode & unix.S_IFMT
	if openErr == nil && mode != unix.S_IFREG && mode != unix.S_IFDIR {
		return true, p.unlinkByName(fd, name, want)
	}
	if openErr != nil && !errors.Is(openErr, unix.ELOOP) && !errors.Is(openErr, unix.EOPNOTSUPP) {
		return false, nil
	}
	if openErr != nil {
		return true, p.unlinkByName(fd, name, want)
	}
	return false, nil
}
