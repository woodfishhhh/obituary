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
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return MetadataStamp{}, err
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
