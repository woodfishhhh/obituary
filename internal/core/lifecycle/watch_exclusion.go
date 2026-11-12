package lifecycle

import (
	"errors"
	"fmt"
	"slices"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

type WatchExclusion = ledger.WatchExclusion

func (s *Service) checkWatchExclusions(p WatchPolicy, path string, id Identity) error {
	for _, e := range p.Exclusions {
		blocked := sameWatchIdentity(e.Identity, id) || samePath(path, e.Path) || isInside(e.Path, path)
		if !blocked {
			current, err := s.FS.Locate(e.Identity)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return err
			}
			blocked = err == nil && (samePath(path, current) || isInside(current, path))
		}
		if blocked {
			return &RefusedError{Path: path, Reason: fmt.Sprintf("excluded folder #%d; explicitly clear its watch exclusion to resume", e.ID)}
		}
	}
	return nil
}

// ClearWatchExclusion explicitly allows an ancestor to manage this unit again.
func (s *Service) ClearWatchExclusion(policyID, exclusionID int64) error {
	return s.Store.UpdateHot(func(st *State) error {
		idx := slices.IndexFunc(st.Watches, func(p WatchPolicy) bool { return p.ID == policyID })
		if idx < 0 {
			return fmt.Errorf("no folder policy #%d", policyID)
		}
		p := &st.Watches[idx]
		i := slices.IndexFunc(p.Exclusions, func(e WatchExclusion) bool { return e.ID == exclusionID })
		if i < 0 {
			return fmt.Errorf("no exclusion #%d on policy #%d", exclusionID, policyID)
		}
		p.Exclusions = slices.Delete(p.Exclusions, i, i+1)
		reviseScope(p)
		return nil
	})
}
