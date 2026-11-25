//go:build linux

package winfs

import "golang.org/x/sys/unix"

func restoreParentPathOpenFlags() int {
	return unix.O_PATH | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
}

func restoreParentChainOpenFlags() int {
	return restoreParentPathOpenFlags()
}

func restoreParentChainFollowFlags() int {
	return unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC
}

func restoreParentPath(fd int, fallback string) (string, error) {
	return fallback, nil
}

func restoreParentPathMatches(fd int, path string) error {
	return nil
}
