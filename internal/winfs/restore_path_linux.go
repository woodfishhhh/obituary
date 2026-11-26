//go:build linux

package winfs

import "golang.org/x/sys/unix"

const restoreParentAccess = unix.O_PATH

func restoreParentPath(fd int, fallback string) (string, error) {
	return fallback, nil
}

func restoreParentPathMatches(fd int, path string) error {
	return nil
}
