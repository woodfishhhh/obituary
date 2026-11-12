package winfs

import "golang.org/x/sys/unix"

func renameExclusive(from int, name string, to int, target string) error {
	return unix.RenameatxNp(from, name, to, target, unix.RENAME_EXCL)
}

const moveOpenFlags = unix.O_EVTONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
