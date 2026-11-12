package lifecycle

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
)

func (s *Service) watchGuard() Guard {
	g := s.Guard
	g.Keep = slices.DeleteFunc(slices.Clone(g.Keep), func(kept string) bool {
		return slices.ContainsFunc(g.WatchAllowed, func(allowed string) bool { return samePath(kept, allowed) })
	})
	return g
}

// WatchPath verifies the configured name still denotes the approved directory.
// Directory replacement or a new reparse target needs a new explicit policy.
func (s *Service) WatchPath(p WatchPolicy) (string, error) {
	path, id, err := s.watchDirectory(p.Path)
	if err != nil {
		return "", err
	}
	if id != p.Identity {
		return "", fmt.Errorf("watched directory was replaced: %w", ErrUnsafe)
	}
	return path, nil
}

// Called under store.lock for registration and immediately before moving.
func (s *Service) checkWatchedFolders(st *State, path string, identity Identity, isDir bool) error {
	if !isDir || len(st.Watches) == 0 {
		return nil
	}
	real, err := s.FS.RealPath(path)
	if err != nil {
		return err
	}
	for _, p := range st.Watches {
		if identity == p.Identity || samePath(real, p.Path) || isInside(p.Path, real) {
			return &RefusedError{Path: path, Reason: "it is or contains a watched folder"}
		}
		current := p.Path
		got, err := s.FS.Identify(current)
		if err != nil || got != p.Identity {
			current, err = s.FS.Locate(p.Identity)
		}
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if samePath(real, current) || isInside(current, real) {
			return &RefusedError{Path: path, Reason: "it is or contains a watched folder"}
		}
	}
	return nil
}

func (s *Service) checkWatchScope(st *State, it Item, path, reason string) error {
	if !s.watchMove(it, reason) {
		return nil
	}
	idx := slices.IndexFunc(st.Watches, func(p WatchPolicy) bool { return p.ID == it.WatchID })
	if idx >= 0 && st.Watches[idx].Consent == "pending" {
		return fmt.Errorf("folder policy awaits approval; existing lifespan paused")
	}
	if idx < 0 || st.Watches[idx].Consent != ConsentApproved {
		return &RefusedError{Path: path, Reason: "folder policy is no longer active"}
	}
	p := st.Watches[idx]
	if it.Identity == nil || watchExempt(st, p.ID, *it.Identity) {
		return &RefusedError{Path: path, Reason: "object was explicitly spared or restored"}
	}
	_, err := s.WatchPath(p)
	if err != nil {
		return err
	}
	real, err := s.FS.RealPath(path)
	if err != nil {
		return err
	}
	parent, err := s.FS.IdentifyFollowing(filepath.Dir(real))
	if err != nil {
		return err
	}
	if !sameWatchIdentity(parent, p.Identity) {
		return &RefusedError{Path: path, Reason: "it left the watched folder"}
	}
	return s.checkWatchExclusions(p, path, *it.Identity)
}
