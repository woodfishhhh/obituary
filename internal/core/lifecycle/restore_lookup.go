package lifecycle

import (
	"errors"
	"path/filepath"
)

// POSIX needs a parent spelling because it cannot reopen a directory by inode.
// The backend verifies the supplied spelling against the persisted identity;
// missing or renamed parents retain recovery evidence instead of guessing.
func (s *Service) findRestored(b Burial, intent *restoreIntent) (string, error) {
	if fs, ok := s.FS.(interface {
		FindRestoredIn(string, Identity, Identity) (string, error)
	}); ok {
		return fs.FindRestoredIn(filepath.Dir(b.Path), intent.Parent, *b.Identity)
	}
	return s.FS.FindRestored(intent.Parent, *b.Identity)
}

// LocateKnown uses a remembered spelling only if it still names the same object.
// A missing or replaced spelling cannot establish that an inode is gone.
func (s *Service) LocateKnown(path string, want Identity) (string, error) {
	current, err := s.FS.Locate(want)
	if !errors.Is(err, ErrUnsupported) {
		return current, err
	}
	if got, checkErr := s.FS.Identify(path); checkErr == nil && got == want {
		return s.FS.RealPath(path)
	}
	return "", err
}

func (s *Service) findBuriedChild(b Burial, want Identity) (string, error) {
	current, err := s.FS.Locate(want)
	if errors.Is(err, ErrUnsupported) {
		if fs, ok := s.FS.(interface {
			LocateIn(string, Identity) (string, error)
		}); ok {
			return fs.LocateIn(b.Grave, want)
		}
	}
	return current, err
}
