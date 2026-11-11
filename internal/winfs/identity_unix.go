//go:build linux || darwin

package winfs

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/unix"
)

// FS retains the existing import boundary while selecting a POSIX backend.
type FS struct{}

var _ corefs.FS = FS{}

func inodeBytes(st *unix.Stat_t) [16]byte {
	var id [16]byte
	binary.BigEndian.PutUint64(id[8:], st.Ino)
	return id
}

func identity(st *unix.Stat_t, path string) (corefs.Identity, error) {
	volume, err := pathutil.MountRoot(path)
	if err != nil {
		return corefs.Identity{}, err
	}
	var mount unix.Stat_t
	if err := unix.Stat(volume, &mount); err != nil {
		return corefs.Identity{}, err
	}
	if mount.Dev != st.Dev {
		return corefs.Identity{}, fmt.Errorf("mount changed while identifying %s; retry", path)
	}
	id := inodeBytes(st)
	return corefs.Identity{Volume: volume, Serial: uint64(st.Dev), FileID: hex.EncodeToString(id[:])}, nil
}

func (FS) Identify(path string) (corefs.Identity, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return corefs.Identity{}, mapNotFound(err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR && st.Nlink > 1 {
		return corefs.Identity{}, fmt.Errorf("multiple hard links: %w", corefs.ErrUnsafe)
	}
	return identity(&st, path)
}

func IdentifyFollowing(path string) (corefs.Identity, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return corefs.Identity{}, err
	}
	var st unix.Stat_t
	if err := unix.Stat(real, &st); err != nil {
		return corefs.Identity{}, err
	}
	return identity(&st, real)
}

func (FS) IdentifyFollowing(path string) (corefs.Identity, error) { return IdentifyFollowing(path) }
func Resolve(path string) (string, error)                         { return filepath.EvalSymlinks(path) }
func (FS) Resolve(path string) (string, error)                    { return Resolve(path) }

func Canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(abs); err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

func (FS) RealPath(path string) (string, error) { return Canonical(path) }
func (FS) LongPath(path string) (string, bool, error) {
	p, err := pathutil.Abs(path)
	return p, err == nil, err
}
func (FS) SameName(a, b string) (bool, error) { return a == b, nil }

// POSIX has no unprivileged, persistent open-by-(dev,inode) primitive. A scan
// cannot prove absence across unreadable directories or concurrent renames.
func (FS) Locate(corefs.Identity) (string, error) {
	return "", fmt.Errorf("locating a moved inode is unavailable; tracked entry retained: %w", corefs.ErrUnsupported)
}

func mapNotFound(err error) error {
	if errors.Is(err, unix.ENOENT) {
		return corefs.ErrNotFound
	}
	return err
}
