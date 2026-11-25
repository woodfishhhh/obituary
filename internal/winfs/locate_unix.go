//go:build linux || darwin

package winfs

import (
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

// Inspect the enumerated name relative to the pinned directory, using the same
// inode generation as registered identities. A mount boundary is never a child.
func (backend FS) BurialEntry(parent fs.ReadDirFile, path string, dir corefs.Identity) (corefs.Identity, bool, error) {
	fd := int(parent.(*os.File).Fd())
	st, err := backend.burialStat(fd, filepath.Base(path))
	if err != nil {
		return corefs.Identity{}, false, err
	}
	isDir := st.Mode&unix.S_IFMT == unix.S_IFDIR
	if !isDir && st.Nlink > 1 {
		return corefs.Identity{}, false, corefs.ErrUnsafe
	}
	id := inodeBytes(&st)
	child := corefs.Identity{Volume: dir.Volume, Serial: st.Dev, FileID: hex.EncodeToString(id[:])}
	return child, isDir, child.RequirePersistent()
}
