//go:build linux || darwin

package winfs

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

// FindRestoreDirectory checks siblings first, then their immediate children.
// WalkDir sorts entire directories; streaming names through pinned descriptors
// bounds the work and lets an unreadable sibling be skipped independently.
func (fs FS) FindRestoreDirectory(root corefs.RestoreParent, want corefs.Identity) string {
	p, ok := root.(*restoreParent)
	if !ok || p.id.Serial != want.Serial || want.RequirePersistent() != nil {
		return ""
	}
	s := directorySearch{fs: fs, want: want, left: 512, until: time.Now().Add(100 * time.Millisecond)}
	found := s.scan(p.file, p.path, true)
	for _, name := range s.children {
		if found != "" || s.exhausted() {
			break
		}
		child, err := openHintDirectory(p.fd(), name)
		if err != nil {
			continue
		}
		f := os.NewFile(uintptr(child), name)
		if st, err := fs.statAt(child, ""); err == nil && st.Dev == want.Serial {
			found = s.scan(f, filepath.Join(p.path, name), false)
		}
		f.Close()
	}
	if found == "" || time.Now().After(s.until) {
		return ""
	}
	if back, err := fs.Identify(p.path); err != nil || back != p.id {
		return ""
	}
	if back, err := fs.Identify(found); err != nil || back != want {
		return ""
	}
	return found
}

type directorySearch struct {
	fs       FS
	want     corefs.Identity
	left     int
	until    time.Time
	children []string
}

func (s *directorySearch) exhausted() bool { return s.left == 0 || time.Now().After(s.until) }

func (s *directorySearch) scan(dir *os.File, path string, siblings bool) string {
	for !s.exhausted() {
		names, err := dir.Readdirnames(min(32, s.left))
		for _, name := range names {
			if s.exhausted() {
				return ""
			}
			s.left--
			st, err := s.fs.statAt(int(dir.Fd()), name)
			if err != nil || st.Mode&unix.S_IFMT != unix.S_IFDIR || st.Dev != s.want.Serial {
				continue
			}
			id := inodeBytes(&st)
			if hex.EncodeToString(id[:]) == s.want.FileID {
				// A same-device bind mount can expose the same inode under a different name.
				fd, err := openHintDirectory(int(dir.Fd()), name)
				if err == nil {
					unix.Close(fd)
					return filepath.Join(path, name)
				}
			}
			if siblings {
				s.children = append(s.children, name)
			}
		}
		if err != nil {
			break
		}
	}
	return ""
}
