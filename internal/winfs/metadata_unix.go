//go:build linux || darwin

package winfs

import (
	"fmt"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

type MetadataStamp struct {
	Serial    uint64
	FileID    [16]byte
	Size      uint64
	Modified  int64
	Directory bool
}

func ReadMetadata(path string) (MetadataStamp, error) {
	return (FS{}).readMetadata(path)
}

func (fs FS) readMetadata(path string) (MetadataStamp, error) {
	real, err := Resolve(path)
	if err != nil {
		return MetadataStamp{}, err
	}
	st, err := fs.statAt(unix.AT_FDCWD, real)
	if err != nil {
		return MetadataStamp{}, err
	}
	if st.Incarnation == 0 {
		// An inode-only stamp cannot authorize reuse of a cached Git answer.
		return MetadataStamp{}, fmt.Errorf("persistent metadata identity unavailable: %w", corefs.ErrUnsupported)
	}
	return MetadataStamp{Serial: uint64(st.Dev), FileID: inodeBytes(&st), Size: uint64(st.Size), Modified: st.Mtim.Nano(), Directory: st.Mode&unix.S_IFMT == unix.S_IFDIR}, nil
}

func HardlinkNames(path string) ([]string, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return nil, err
	}
	if st.Nlink != 1 {
		return nil, fmt.Errorf("enumerating all hard links is unavailable: %w", corefs.ErrUnsupported)
	}
	return []string{path}, nil
}
