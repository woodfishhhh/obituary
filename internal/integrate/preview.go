package integrate

import (
	"errors"
	"fmt"
	"os"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

type integrationInspection struct {
	paths map[inspectionPath]error
	files map[string]Change
}

type inspectionPath struct {
	path      string
	directory bool
}

// ForInspection shares reads within one diagnostic invocation. Its plans cannot
// be applied. Start with a fresh Scope for the next invocation to observe edits.
func (s Scope) ForInspection() Scope {
	if s.inspection == nil {
		s.inspection = &integrationInspection{paths: map[inspectionPath]error{}, files: map[string]Change{}}
	}
	return s
}

// Inspection reads current bytes and checks links. Executable installation
// plans capture identities and pin directory traversal separately, since a
// read-only preview cannot authorize a later write.
func (s Scope) readIntegrationPreview(ch Change) (Change, error) {
	ch.readOnly = true
	path, err := pathutil.Abs(ch.Path)
	if err != nil {
		return ch, err
	}
	if err := s.checkIntegrationAncestors(path); err != nil {
		return ch, err
	}
	if saved, ok := s.inspection.files[path]; ok {
		ch.orig, ch.existed = saved.orig, saved.existed
		return ch, nil
	}
	data, err := os.ReadFile(path)
	ch.orig, ch.existed = data, err == nil
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err == nil {
		s.inspection.files[path] = ch
	}
	return ch, err
}

// os.Lstat uses GetFileAttributesEx for ordinary Windows paths; unlike
// Root.Lstat it does not open and stat a handle at every ancestor. This is a
// planning check only: Apply always traverses again through pinned os.Roots.
func (s Scope) checkIntegrationAncestors(path string) error {
	return s.checkIntegrationPath(path, false)
}

func (s Scope) checkIntegrationPath(path string, directory bool) (err error) {
	if s.inspection != nil {
		key := inspectionPath{path, directory}
		if saved, ok := s.inspection.paths[key]; ok {
			return saved
		}
		defer func() { s.inspection.paths[key] = err }()
	}
	for {
		if err := inspectIntegrationEntry(path, directory); err != nil {
			return err
		}
		parent := pathutil.Parent(path)
		if parent == path {
			return nil
		}
		if s.inspection != nil {
			return s.checkIntegrationPath(parent, true)
		}
		path, directory = parent, true
	}
}

func inspectIntegrationEntry(path string, directory bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err == nil {
		err = ordinaryIntegrationEntry(info)
		if err == nil && directory && !info.IsDir() {
			err = fmt.Errorf("not a directory")
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
