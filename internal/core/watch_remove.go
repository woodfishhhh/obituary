package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// RemoveWatch resolves extant aliases by identity, retaining exact saved names
// for missing directories. A failed identity read requires an explicit ID.
func (s *Service) RemoveWatch(path string) error {
	path, _, err := s.FS.LongPath(path)
	if err != nil {
		return err
	}
	real, resolveErr := s.FS.RealPath(path)
	id, identifyErr := s.FS.Identify(path)
	missing := errors.Is(identifyErr, ErrNotFound) || errors.Is(identifyErr, os.ErrNotExist)
	if identifyErr != nil && !missing {
		return identifyErr
	}
	return s.Store.UpdateHot(func(st *State) error {
		idx := -1
		for i, p := range st.Watches {
			match := identifyErr == nil && sameWatchIdentity(id, p.Identity)
			if missing {
				match = path == p.Path || slices.Contains(p.Aliases, path) || resolveErr == nil && real == p.Path
			}
			if !match {
				continue
			}
			if idx >= 0 {
				return fmt.Errorf("ambiguous folder policy; remove by ID")
			}
			idx = i
		}
		if idx < 0 {
			return fmt.Errorf("no folder policy for %s; obit watch --remove-id can remove stale policies", path)
		}
		return s.removeWatch(st, idx)
	})
}

// RemoveWatchID also works when the original directory cannot be resolved.
func (s *Service) RemoveWatchID(id int64) error {
	return s.Store.UpdateHot(func(st *State) error {
		idx := slices.IndexFunc(st.Watches, func(p WatchPolicy) bool { return p.ID == id })
		if idx < 0 {
			return fmt.Errorf("no folder policy #%d", id)
		}
		return s.removeWatch(st, idx)
	})
}

func (s *Service) removeWatch(st *State, idx int) error {
	removed := st.Watches[idx]
	// Persist the exclusion and revoke all old claims in the same commit.
	for i := range st.Watches {
		p := &st.Watches[i]
		if p.ID == removed.ID {
			continue
		}
		inside, err := s.watchAncestor(removed.Path, p.Identity)
		if err != nil || isInside(removed.Path, p.Path) {
			inside = true
		}
		if inside && !slices.ContainsFunc(p.Exclusions, func(e WatchExclusion) bool { return e.ID == removed.ID }) {
			p.Exclusions = append(p.Exclusions, WatchExclusion{ID: removed.ID, Path: removed.Path, Identity: removed.Identity})
			p.reviseScope()
		}
	}
	st.Items = slices.DeleteFunc(st.Items, func(it Item) bool { return it.WatchID == removed.ID })
	st.Proposals = slices.DeleteFunc(st.Proposals, func(p Proposal) bool { return p.WatchID == removed.ID })
	st.WatchExemptions = slices.DeleteFunc(st.WatchExemptions, func(e WatchExemption) bool { return e.WatchID == removed.ID })
	st.Watches = slices.Delete(st.Watches, idx, idx+1)
	return nil
}

func (s *Service) watchAncestor(path string, ancestor Identity) (bool, error) {
	for parent := filepath.Dir(path); parent != path; parent = filepath.Dir(path) {
		id, err := s.FS.IdentifyFollowing(parent)
		if err != nil {
			return false, err
		}
		if sameWatchIdentity(id, ancestor) {
			return true, nil
		}
		path = parent
	}
	return false, nil
}
