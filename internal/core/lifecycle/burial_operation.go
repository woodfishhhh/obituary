package lifecycle

import (
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

type BurialOperation = ledger.BurialOperation

// BeginBurial returns a request-local service sharing one deletion batch.
// Call it once per command; plain Bury creates a new unscoped batch per call.
func (s *Service) BeginBurial(session, project string) (*Service, error) {
	if err := ValidateMetadata("session", session); err != nil {
		return nil, err
	}
	if project != "" {
		var err error
		project, err = s.referencePath(project)
		if err != nil {
			return nil, err
		}
	}
	copy := *s
	copy.burialOperation = &BurialOperation{ID: rand.Text(), StartedAt: s.now(), Session: session, Project: project}
	return &copy, nil
}

// LastBurialRefs chooses IDs and returns receipts reconciled during this read;
// Resurrect still verifies and restores the remaining IDs. Selection requires
// an explicit operation in the caller's session or project to avoid restoring
// an unrelated deletion from another context.
func (s *Service) LastBurialRefs(session, project string) (refs []string, restored []GraveResult, err error) {
	if err := ValidateMetadata("session", session); err != nil {
		return nil, nil, err
	}
	view, err := s.ReadGraveyard()
	if err != nil {
		return nil, nil, err
	}
	candidates := burialOperationCandidates(view)
	var selected []Burial
	if session != "" {
		for _, b := range candidates {
			if b.Operation.Session == session {
				selected = append(selected, b)
			}
		}
	}
	if len(selected) == 0 && project != "" {
		selected, err = s.projectBurialOperations(candidates, session, project)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(selected) == 0 {
		return nil, nil, errors.New("no deletion batch in the current session or project; use obit graveyard to select IDs")
	}
	latest := latestBurialOperation(selected)
	var ids []int64
	for _, b := range selected {
		if b.Operation.ID == latest.ID {
			if i := slices.IndexFunc(view.Restored, func(r GraveResult) bool { return r.Burial.ID == b.ID }); i >= 0 {
				restored = append(restored, view.Restored[i])
				continue
			}
			if !slices.ContainsFunc(view.Burials, func(recorded Burial) bool { return recorded.ID == b.ID }) {
				return nil, nil, errors.New("the latest batch has pending burial recovery; run obit reap, then retry --last; obit graveyard shows pending records")
			}
			ids = append(ids, b.ID)
		} else if b.Operation.StartedAt.Equal(latest.StartedAt) {
			return nil, nil, fmt.Errorf("deletion batches have the same start time; use obit graveyard to select IDs")
		}
	}
	slices.Sort(ids)
	refs = make([]string, len(ids))
	for i, id := range ids {
		refs[i] = strconv.FormatInt(id, 10)
	}
	return refs, restored, nil
}

func latestBurialOperation(selected []Burial) *BurialOperation {
	latest := selected[0].Operation
	for _, b := range selected {
		if b.Operation.StartedAt.After(latest.StartedAt) {
			latest = b.Operation
		}
	}
	return latest
}

func burialOperationCandidates(view *GraveyardView) []Burial {
	burials := slices.Clone(view.Burials)
	// Reconciliation can remove the last live member of the latest batch.
	// Keep its receipt in this selection so retry cannot fall back to an older batch.
	for _, r := range view.Restored {
		burials = append(burials, *r.Burial)
	}
	for _, p := range view.Pending {
		if p.Burial != nil && !slices.ContainsFunc(burials, func(b Burial) bool { return b.ID == p.Burial.ID }) {
			burials = append(burials, *p.Burial)
		}
	}
	var candidates []Burial
	for _, b := range burials {
		if b.Reason == ReasonBury && b.Operation != nil && b.Operation.ID != "" && !b.Operation.StartedAt.IsZero() {
			candidates = append(candidates, b)
		}
	}
	return candidates
}

func (s *Service) projectBurialOperations(candidates []Burial, session, project string) ([]Burial, error) {
	project, err := s.referencePath(project)
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(candidates))
	for i, b := range candidates {
		// An agent only falls back to unscoped batches. A human terminal
		// with no session can undo any session's latest project batch.
		if session == "" || b.Operation.Session == "" {
			paths[i] = b.Operation.Project
		}
	}
	matches, exact := matchingPaths(paths, project)
	var selected []Burial
	for _, i := range matches {
		if !exact && paths[i] != paths[matches[0]] {
			return nil, errors.New("project matches multiple spellings; use obit graveyard to select IDs")
		}
		selected = append(selected, candidates[i])
	}
	return selected, nil
}
