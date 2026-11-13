package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func (s *Service) prepareRestoreParent(b Burial, dest string) (_ *restoreParentChain, err error) {
	if reason := s.Guard.ProtectedReason(dest); reason != "" {
		return nil, &RefusedError{Path: dest, Reason: reason}
	}
	dir := filepath.Dir(dest)
	nearest, missing, err := existingRestoreAncestor(dir)
	if err != nil {
		return nil, err
	}
	// Resolve supplies only a spelling to pin. Validate the original chain
	// after pinning and before the first handle-relative directory creation.
	real, err := s.FS.Resolve(nearest)
	if err != nil {
		return nil, err
	}
	parent, err := s.FS.OpenRestoreParent(real)
	if err != nil {
		return nil, err
	}
	parents := []RestoreParent{parent}
	defer func() {
		for i := len(parents) - 1; i >= 0; i-- {
			if err != nil {
				err = errors.Join(err, parents[i].Rollback())
			}
			if err != nil {
				parents[i].Close()
			}
		}
	}()
	if err := s.preflightRestoreParent(b, dest, nearest, parent); err != nil {
		return nil, err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		child, err := s.FS.MkdirRestoreParent(parent, missing[i])
		if err != nil {
			return nil, err
		}
		parents = append(parents, child)
		parent = child
	}
	// Pin first, then check the entire original lexical chain, then prove that
	// this checked spelling still reaches that handle (not a freshly resolved
	// replacement). Ordinary rebuilt parents keep their original path contract.
	if err := s.checkCreatedRestoreParent(b, dest, dir, parent); err != nil {
		return nil, err
	}
	return &restoreParentChain{RestoreParent: parent, parents: parents}, nil
}

func existingRestoreAncestor(dir string) (string, []string, error) {
	nearest := dir
	var missing []string
	for {
		info, err := os.Lstat(nearest)
		if err == nil && info.Mode().IsRegular() {
			return "", nil, fmt.Errorf("original folder %s is now a file; the item is still in the graveyard, and after that file is moved or renamed, obit resurrect can restore it", nearest)
		}
		if err == nil {
			break
		}
		// POSIX reports ENOTDIR below a file; keep walking up to name that file.
		if !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTDIR) || pathutil.Parent(nearest) == nearest {
			return "", nil, err
		}
		missing = append(missing, filepath.Base(nearest))
		nearest = pathutil.Parent(nearest)
	}
	return nearest, missing, nil
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
		return fmt.Errorf("original parent %s no longer names the pinned directory: %w", dir, ErrUnsafe)
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
