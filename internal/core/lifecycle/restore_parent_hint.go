package lifecycle

import (
	"errors"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func restorePreparationError(path string, err error) error {
	if _, ok := errors.AsType[*corefs.RestoreParentError](err); ok {
		return err
	}
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		path = pe.Path
	}
	kind := ""
	switch {
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
		if rec.Reparse {
			if id, err := s.FS.IdentifyFollowing(rec.Path); err != nil || id != rec.Identity {
				issue.Path, issue.Kind = rec.Path, "link"
			}
			break
		}
		if i+1 == len(parents) || filepath.Dir(rec.Path) != parents[i+1].Path {
			break
		}
		upper := parents[i+1]
		root, err := s.FS.OpenRestoreParent(upper.Path)
		if err != nil {
			continue
		}
		matched := root.Identity() == upper.Identity && upper.Identity.RequirePersistent() == nil
		var found string
		if matched {
			found = locator.FindRestoreDirectory(root, rec.Identity)
		}
		root.Close()
		if !matched {
			continue
		}
		if found != "" && found != rec.Path {
			s.describeRenamedParent(issue, parents[:i+1], found)
		}
		break
	}
	result.Error = issue.Error()
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
		next := filepath.Join(issue.Current, filepath.Base(child.Path))
		if id, err := s.FS.Identify(next); err != nil || id != child.Identity || id.RequirePersistent() != nil {
			break
		}
		issue.Path, issue.Current = child.Path, next
	}
}
