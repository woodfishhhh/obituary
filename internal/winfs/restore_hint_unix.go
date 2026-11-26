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
// bounds entries and checks elapsed time between system calls.
func (fs FS) FindRestoreDirectory(root corefs.RestoreParent, want corefs.Identity) string {
	p, ok := root.(*restoreParent)
	if !ok || p.id.Serial != want.Serial || want.RequirePersistent() != nil {
		return ""
	}
	s := directorySearch{fs: fs, want: want, left: 512, until: time.Now().Add(100 * time.Millisecond)}
	dir, err := p.openReadable()
	if err != nil {
		return ""
	}
	defer dir.Close()
	found := s.scan(dir, p.path, true)
	for _, name := range s.children {
		if found != "" || s.exhausted() {
			break
		}
		child, err := s.openDirectory(p.file, p.path, name, unix.O_RDONLY)
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
		entries, err := readHintEntries(dir, min(32, s.left))
		for _, entry := range entries {
			if !entry.directory || entry.mountPoint {
				continue
			}
			name := entry.name
			if s.exhausted() {
				return ""
			}
			s.left--
			st, err := s.directoryStat(dir, path, name)
			if err != nil || st.Dev != s.want.Serial {
				continue
			}
			id := inodeBytes(&st)
			if hex.EncodeToString(id[:]) == s.want.FileID {
				return filepath.Join(path, name)
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

func (s *directorySearch) openDirectory(parent *os.File, path, name string, access int) (int, error) {
	return openHintDirectory(int(parent.Fd()), name, access)
}

func (s *directorySearch) directoryStat(parent *os.File, path, name string) (posixStat, error) {
	fd, err := s.openDirectory(parent, path, name, hintMetadataOnly)
	if err != nil {
		return posixStat{}, err
	}
	defer unix.Close(fd)
	return s.fs.statAt(fd, "")
}
