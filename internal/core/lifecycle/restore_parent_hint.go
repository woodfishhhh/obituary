package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func restorePreparationError(path string, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*corefs.RestoreParentError](err); ok {
		return err
	}
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		path = pe.Path
	}
	kind := ""
	switch {
	case errors.Is(err, restoreLinkLoop):
		kind = "loop"
	case errors.Is(err, syscall.ENOTDIR):
		kind = "not_directory"
	case errors.Is(err, os.ErrPermission):
		kind = "permission"
	case errors.Is(err, os.ErrNotExist):
		kind = "missing"
	default:
		return err
	}
	return &corefs.RestoreParentError{Kind: kind, Path: path, Cause: err}
}

// This read-only lookup runs after the graveyard lock is released. Only a
// recorded ordinary directory can supply a verified new spelling.
func (s *Service) diagnoseRestoreParent(result *GraveResult) {
	issue, ok := errors.AsType[*corefs.RestoreParentError](result.Err)
	if !ok || issue.Kind != "unverified" || result.Burial == nil {
		return
	}
	locator, ok := s.FS.(interface {
		FindRestoreDirectory(RestoreParent, Identity) string
	})
	if !ok {
		return
	}
	parents := result.Burial.Parents
	for i, rec := range parents {
		if rec.Path != issue.Path || rec.Reparse || i+1 >= len(parents) {
			continue
		}
		upper := parents[i+1]
		if filepath.Dir(rec.Path) != upper.Path || upper.Identity.RequirePersistent() != nil {
			break
		}
		real, err := s.FS.Resolve(upper.Path)
		if err != nil {
			break
		}
		root, err := s.FS.OpenRestoreParent(real)
		if err != nil {
			break
		}
		defer root.Close()
		if root.Identity() != upper.Identity {
			break
		}
		found := locator.FindRestoreDirectory(root, rec.Identity)
		if found != "" {
			rel, err := filepath.Rel(root.Path(), found)
			if err != nil {
				break
			}
			logical := filepath.Join(upper.Path, rel)
			if id, err := s.FS.Identify(logical); logical != rec.Path && err == nil && id == rec.Identity && !s.originalRestoreNameMatches(rec) {
				s.describeRenamedParent(issue, parents[:i+1], logical)
			}
		}
		break
	}
	result.Error = issue.Error()
}

// Recheck outside the lock: a directory can return while the search is in progress.
func (s *Service) originalRestoreNameMatches(rec BurialParent) bool {
	id, err := s.FS.Identify(rec.Path)
	if err != nil || id != rec.Identity {
		return false
	}
	exact, err := exactRestoreName(rec.Path)
	return err != nil || exact
}

// A renamed ancestor can carry an already recorded suffix beyond the search depth.
func (s *Service) describeRenamedParent(issue *corefs.RestoreParentError, parents []BurialParent, found string) {
	root := parents[len(parents)-1]
	issue.Kind, issue.Path, issue.Current = "renamed", root.Path, found
	for j := len(parents) - 2; j >= 0; j-- {
		child := parents[j]
		if child.Reparse || child.Identity.Serial != root.Identity.Serial || filepath.Dir(child.Path) != issue.Path {
			break
		}
		if id, err := s.FS.Identify(child.Path); err == nil && id == child.Identity {
			break
		}
		next := filepath.Join(issue.Current, filepath.Base(child.Path))
		if id, err := s.FS.Identify(next); err != nil || id != child.Identity || id.RequirePersistent() != nil {
			break
		}
		issue.Path, issue.Current = child.Path, next
	}
}
