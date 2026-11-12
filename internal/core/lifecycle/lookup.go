package lifecycle

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core/internal/pathcheck"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

// find resolves IDs directly and paths by their current identity. Only an
// absent object permits a fallback to the ledger's (possibly stale) path.
func (s *Service) find(st *State, ref string) (int, string, error) {
	if id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64); err == nil {
		if idx := st.Index(id); idx >= 0 {
			return idx, st.Items[idx].Path, nil
		}
		if strings.HasPrefix(ref, "#") {
			return -1, ref, nil // explicit IDs never fall back to a cwd-relative filename
		}
	}
	abs, err := s.referencePath(ref)
	if err != nil {
		return -1, "", err
	}
	id, err := s.FS.Identify(abs)
	if err == nil {
		return st.FindIdentity(id), filepath.Clean(abs), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return -1, "", fmt.Errorf("identify %s: %w", abs, err)
	}
	idx, err := st.FindPath(abs)
	return idx, filepath.Clean(abs), err
}

// Resolve aliases once at the service boundary; State only matches strings.
// Missing suffixes keep their spelling, while inaccessible aliases need an ID.
func (s *Service) referencePath(ref string) (string, error) {
	path, complete, err := s.FS.LongPath(ref)
	if err != nil {
		return "", fmt.Errorf("resolve reference %s; use an ID if its alias is unavailable: %w", ref, err)
	}
	if !complete {
		return "", fmt.Errorf("cannot verify every name in %s; use an ID", ref)
	}
	return path, nil
}

func (s *Service) findBurial(st *State, ref string) (int, error) {
	if _, numeric, err := BurialReferenceID(ref); numeric || err != nil {
		if err := s.Store.LoadGraveyardForReference(st, ref); err != nil {
			return -1, err
		}
		return st.FindBurial(ref)
	}
	path, err := s.referencePath(ref)
	if err != nil {
		return -1, err
	}
	if err := s.Store.LoadGraveyardForReference(st, path); err != nil {
		return -1, err
	}
	return st.FindBurial(path)
}

func BurialReferenceID(ref string) (id int64, numeric bool, err error) {
	return ledger.BurialReferenceID(ref)
}

func missingBurial(ref string) error {
	if id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64); err == nil {
		return fmt.Errorf("burial %d was not found; for a filename use ./%s or an absolute path", id, strings.TrimPrefix(ref, "#"))
	}
	return fmt.Errorf("%s is not in the graveyard; if an alias cannot be resolved, use an ID", ref)
}

func matchingPaths(paths []string, path string) ([]int, bool) {
	return pathcheck.MatchingPaths(paths, path)
}
