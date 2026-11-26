//go:build darwin

package winfs

import (
	"bytes"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

const restoreParentAccess = unix.O_SEARCH

func restoreParentPath(fd int, fallback string) (string, error) {
	buf := make([]byte, unix.PathMax)
	_, _, errno := unix.Syscall(unix.SYS_FCNTL, uintptr(fd), uintptr(unix.F_GETPATH), uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return "", errno
	}
	if end := bytes.IndexByte(buf, 0); end >= 0 {
		buf = buf[:end]
	}
	return string(buf), nil
}

func restoreParentPathMatches(fd int, path string) error {
	actual, err := restoreParentPath(fd, path)
	if err != nil {
		return err
	}
	if filepath.Clean(actual) != filepath.Clean(path) {
		return fmt.Errorf("directory spelling changed from %s to %s", path, actual)
	}
	return nil
}
