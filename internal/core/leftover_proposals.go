package core

import (
	"slices"
)

// Keep the first session's review intact and check under the store lock so
// concurrent finishes cannot each publish the same candidate.
func (s *Service) leftoverUnavailable(st *State, project, session string, c Candidate) bool {
	for _, it := range st.Items {
		if it.Kind != KindContainer && it.Identity != nil && samePath(it.Path, c.Path) {
			id, err := s.FS.Identify(c.Path)
			if err == nil && id.Serial == it.Identity.Serial && id.FileID == it.Identity.FileID {
				return true
			}
		}
	}
	pending := false
	for _, p := range st.Proposals {
		if p.Kind != ProposalLeftovers || !samePath(p.Project, project) {
			continue
		}
		if slices.ContainsFunc(p.Candidates, func(have Candidate) bool { return sameCandidate(have, c) }) {
			if p.Session == session {
				return false
			}
			pending = true
		}
	}
	return pending
}

// Legacy proposals may share a path with different local revision numbers.
// Remove only twins of candidates whose own decision actually settled.
func removeLeftoverTwins(st *State, decided Proposal, settled []Candidate) {
	if decided.Kind != ProposalLeftovers || len(settled) == 0 {
		return
	}
	for i := len(st.Proposals) - 1; i >= 0; i-- {
		p := &st.Proposals[i]
		if p.ID == decided.ID || p.Kind != ProposalLeftovers || !samePath(p.Project, decided.Project) {
			continue
		}
		p.Candidates = slices.DeleteFunc(p.Candidates, func(c Candidate) bool {
			return slices.ContainsFunc(settled, func(done Candidate) bool { return sameCandidate(done, c) })
		})
		if len(p.Candidates) == 0 {
			st.Proposals = slices.Delete(st.Proposals, i, i+1)
		}
	}
}
