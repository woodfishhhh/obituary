//go:build linux || darwin

package winfs

import (
	"io/fs"
	"os"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

func (backend FS) OpenBurialDirectory(path string, want corefs.Identity) (fs.ReadDirFile, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	st, err := backend.statAt(fd, "")
	if err == nil {
		var got corefs.Identity
		got, err = identity(&st, path)
		if err == nil && got != want {
			err = corefs.ErrUnsafe
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
