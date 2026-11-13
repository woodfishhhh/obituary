package winfs

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"golang.org/x/sys/unix"
)

func readPOSIXStat(fd int, name string) (posixStat, error) {
	st, err := readLinuxMetadata(fd, name)
	if err == nil {
		st.Incarnation, err = linuxCreationID(fd, name, st.Incarnation)
	}
	return st, err
}

func readLinuxMetadata(fd int, name string) (posixStat, error) {
	var st unix.Statx_t
	err := unix.Statx(fd, name, unix.AT_SYMLINK_NOFOLLOW|unix.AT_EMPTY_PATH, unix.STATX_BASIC_STATS|unix.STATX_BTIME, &st)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL) {
		// Older kernels can still supply an export generation without statx.
		return statWithoutBirth(fd, name)
	}
	if err != nil {
		return posixStat{}, err
	}
	var birth uint64
	if st.Mask&unix.STATX_BTIME != 0 {
		birth = birthNanos(st.Btime.Sec, int64(st.Btime.Nsec))
	}
	return posixStat{Dev: unix.Mkdev(st.Dev_major, st.Dev_minor), Ino: st.Ino, Mode: uint32(st.Mode), Nlink: uint64(st.Nlink), Size: int64(st.Size), Mtim: unix.Timespec{Sec: st.Mtime.Sec, Nsec: int64(st.Mtime.Nsec)}, Incarnation: birth}, nil
}

// Birth time alone can repeat when an inode is deleted and recreated within
// one filesystem clock tick. The export handle includes its generation. Keep
// the opaque handle intact as hash input: its layout depends on the filesystem.
// The birth time further separates generations after wraparound. A zero token
// is reserved for filesystems that cannot provide a persistent incarnation.
func linuxCreationID(fd int, name string, birth uint64) (uint64, error) {
	return linuxCreationIDWith(fd, name, birth, unix.NameToHandleAt)
}

func linuxCreationIDWith(fd int, name string, birth uint64, lookup func(int, string, int) (unix.FileHandle, int, error)) (uint64, error) {
	// Identify-only handles can encode a reusable inode with zero generation
	// on unexportable overlayfs. Successful encoding is not a durable identity.
	handle, _, err := lookup(fd, name, unix.AT_EMPTY_PATH)
	if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var prefix [12]byte
	binary.BigEndian.PutUint32(prefix[:4], uint32(handle.Type()))
	binary.BigEndian.PutUint64(prefix[4:], birth)
	sum := sha256.Sum256(append(prefix[:], handle.Bytes()...))
	return binary.BigEndian.Uint64(sum[:8]), nil
}
