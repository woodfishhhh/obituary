package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func (s *Service) prepareRestoreParent(b Burial, dest string) (*restoreParentChain, error) {
	if reason := s.Guard.ProtectedReason(dest); reason != "" {
		return nil, &RefusedError{Path: dest, Reason: reason}
	}
	dir := filepath.Dir(dest)
	nearest, missing, err := s.existingRestoreAncestor(b, dir)
	if err != nil {
		return nil, err
	}
	// Resolve supplies only a spelling to pin. Validate the original chain
	// after pinning and before the first handle-relative directory creation.
	real, err := s.FS.Resolve(nearest)
	if err != nil {
		// EvalSymlinks may return an untyped loop error. Stat supplies the OS cause.
		if _, statErr := os.Stat(nearest); statErr != nil {
			err = statErr
		}
		return nil, restorePreparationError(nearest, err)
	}
	parent, err := s.FS.OpenRestoreParent(real)
	if err != nil {
		return nil, restorePreparationError(nearest, err)
	}
	chain := &restoreParentChain{RestoreParent: parent, parents: []RestoreParent{parent}}
	if err := s.preflightRestoreParent(b, dest, nearest, parent); err != nil {
		return chain, err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		child, err := s.FS.MkdirRestoreParent(parent, missing[i])
		if err != nil {
			return chain, err
		}
		chain.parents = append(chain.parents, child)
		chain.RestoreParent = child
		parent = child
	}
	// Pin first, then check the entire original lexical chain, then prove that
	// this checked spelling still reaches that handle (not a freshly resolved
	// replacement). Ordinary rebuilt parents keep their original path contract.
	if err := s.checkCreatedRestoreParent(b, dest, dir, parent); err != nil {
		return chain, err
	}
	if p, ok := parent.(interface{ BeforeRestoreRename(func() error) }); ok {
		p.BeforeRestoreRename(func() error { return restorePreparationError(dir, s.checkCreatedRestoreParent(b, dest, dir, parent)) })
	}
	return chain, nil
}

func (s *Service) existingRestoreAncestor(b Burial, dir string) (string, []string, error) {
	nearest := dir
	var missing []string
	for {
		info, err := os.Lstat(nearest)
		if err == nil {
			if info.Mode().IsRegular() {
				return s.existingRestoreFile(b, nearest)
			}
			break
		}
		// POSIX reports ENOTDIR below a file; keep walking up to name that file.
		if !missingRestoreAncestor(err, nearest) {
			return "", nil, err
		}
		missing = append(missing, filepath.Base(nearest))
		nearest = pathutil.Parent(nearest)
	}
	return nearest, missing, nil
}

func (s *Service) existingRestoreFile(b Burial, path string) (string, []string, error) {
	if changed, missing, found, err := s.changedRestoreFile(b); found || err != nil {
		return changed, missing, err
	}
	return "", nil, occupiedRestoreParent(path, b.Parents)
}

func (s *Service) changedRestoreFile(b Burial) (string, []string, bool, error) {
	if runtime.GOOS == "windows" {
		return "", nil, false, nil
	}
	changed, err := s.firstChangedRestoreParent(b)
	if err != nil {
		return "", nil, false, err
	}
	if changed == nil {
		return "", nil, false, nil
	}
	if changedInfo, statErr := os.Lstat(changed.Path); statErr == nil && changedInfo.Mode().IsRegular() {
		return "", nil, true, occupiedRestoreParent(changed.Path, b.Parents)
	}
	return "", nil, true, changedRestoreParent(changed.Path, changed)
}

func missingRestoreAncestor(err error, path string) bool {
	return (os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR)) && pathutil.Parent(path) != path
}

func (s *Service) firstChangedRestoreParent(b Burial) (*BurialParent, error) {
	for i := len(b.Parents) - 1; i >= 0; i-- {
		recorded := &b.Parents[i]
		info, err := os.Lstat(recorded.Path)
		if os.IsNotExist(err) {
			return recorded, nil
		}
		if err != nil {
			return nil, err
		}
		if isReparse(info) != recorded.Reparse {
			return recorded, nil
		}
		id, err := s.FS.IdentifyFollowing(recorded.Path)
		if err != nil {
			return nil, err
		}
		if !restoreParentIdentityMatches(recorded, id) {
			return recorded, nil
		}
	}
	return nil, nil
}

func occupiedRestoreParent(path string, parents []BurialParent) error {
	issue := &corefs.RestoreParentError{Kind: "file", Path: path, Cause: syscall.ENOTDIR}
	if runtime.GOOS != "windows" {
		return issue
	}
	for _, rec := range parents {
		if rec.Path != path {
			continue
		}
		issue.Kind = "file_windows"
		if rec.Reparse {
			issue.Kind = "link_file"
		}
		break
	}
	return issue
}

func (s *Service) checkCreatedRestoreParent(b Burial, dest, dir string, parent RestoreParent) error {
	if err := s.checkRestoreParents(b); err != nil {
		return err
	}
	current, err := s.FS.IdentifyFollowing(dir)
	if err != nil {
		return err
	}
	pinned := parent.Identity()
	if current.Serial != pinned.Serial || current.FileID != pinned.FileID {
		return &corefs.RestoreParentError{Kind: "pinned", Path: dir, Cause: ErrUnsafe}
	}
	path := filepath.Join(parent.Path(), filepath.Base(dest))
	if reason := s.Guard.ProtectedReason(path); reason != "" {
		return &RefusedError{Path: path, Reason: reason}
	}
	return nil
}

// Keep creation handles until the rename succeeds, so later refusal can also
// undo empty directories without reopening a path that may now name a junction.
type restoreParentChain struct {
	RestoreParent
	parents []RestoreParent
}

func (p *restoreParentChain) Close() error {
	var err error
	for i := len(p.parents) - 1; i >= 0; i-- {
		err = errors.Join(err, p.parents[i].Close())
	}
	return err
}

func (p *restoreParentChain) Rollback() error {
	var err error
	for i := len(p.parents) - 1; i >= 0; i-- {
		err = errors.Join(err, p.parents[i].Rollback())
		err = errors.Join(err, p.parents[i].Close())
	}
	p.parents = nil
	return err
}
