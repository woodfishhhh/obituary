package lifecycle

import (
	"fmt"
)

type resurrectionCandidates struct {
	matches    State
	committed  map[int64]PendingGrave
	unverified map[int64]string
}

// Path references select once across active burials and committed receipts,
// preserving exact-name, latest-burial and ambiguity rules. Selection failures
// are per-ref, including uncertainty in that ref's own graveyard root.
func (s *Service) findResurrection(state State, ref string) (*Burial, *PendingGrave, error) {
	candidates, err := s.resurrectionCandidates(state, ref)
	if err != nil {
		return nil, nil, err
	}
	matches := &candidates.matches
	match, lookupErr := s.findBurial(matches, ref)
	if lookupErr != nil {
		return nil, nil, lookupErr
	}
	if match < 0 {
		return nil, nil, missingBurial(ref)
	}
	b := &matches.Burials[match]
	if detail, uncertain := candidates.unverified[b.ID]; uncertain {
		return b, nil, fmt.Errorf("cannot verify latest burial #%d for %s; retry later or use an ID: %s", b.ID, ref, detail)
	}
	if p, committed := candidates.committed[b.ID]; committed {
		return b, &p, nil
	}
	return b, nil, nil
}

func (s *Service) resurrectionCandidates(state State, ref string) (*resurrectionCandidates, error) {
	id, numeric, err := BurialReferenceID(ref)
	if err != nil {
		return nil, err
	}
	candidates := &resurrectionCandidates{
		matches:    State{Burials: append([]Burial(nil), state.Burials...)},
		committed:  make(map[int64]PendingGrave),
		unverified: make(map[int64]string),
	}
	if numeric && state.BurialIndex(id) >= 0 {
		return candidates, nil
	}
	pending, scanErr := s.scanManifests(state)
	if !numeric && scanErr != nil {
		if err := s.checkResurrectionScan(ref, pending); err != nil {
			return nil, err
		}
	}
	for _, p := range pending {
		if !numeric && p.Status == "unverifiable" {
			candidates.addUnverified(state, p)
		}
		if p.Status != "restore-committed" {
			continue
		}
		candidates.committed[p.Burial.ID] = p
		candidates.matches.Burials = append(candidates.matches.Burials, *p.Burial)
	}
	return candidates, nil
}

func (s *Service) checkResurrectionScan(ref string, pending []PendingGrave) error {
	path, err := s.referencePath(ref)
	if err != nil {
		return err
	}
	root := s.graveyardRoot(path)
	for _, p := range pending {
		if p.root != "" && samePath(p.root, root) {
			return fmt.Errorf("scan graveyard for %s; retry later or use an ID: %s", ref, p.Error)
		}
	}
	return nil
}

func (c *resurrectionCandidates) addUnverified(state State, p PendingGrave) {
	b := p.Burial
	if idx := cleanupIndex(&state, p.Number()); idx >= 0 {
		b = &state.RestoreCleanups[idx].Burial
	}
	if b == nil {
		if c.matches.BurialIndex(p.Number()) >= 0 {
			c.unverified[p.Number()] = p.Error
		}
		return
	}
	c.unverified[b.ID] = p.Error
	if c.matches.BurialIndex(b.ID) < 0 {
		c.matches.Burials = append(c.matches.Burials, *b)
	}
}
