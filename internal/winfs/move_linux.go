package winfs

import "golang.org/x/sys/unix"

func renameExclusive(from int, name string, to int, target string) error {
	return unix.Renameat2(from, name, to, target, unix.RENAME_NOREPLACE)
}

// O_PATH pins an unreadable inode too; fstat supplies the identity and type.
const moveOpenFlags = unix.O_PATH | unix.O_NOFOLLOW | unix.O_CLOEXEC
