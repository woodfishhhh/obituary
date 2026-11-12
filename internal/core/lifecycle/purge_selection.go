package lifecycle

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// purgeRun keeps selection and receipts local to one graveyard lock.
type purgeRun struct {
	all, dryRun, background bool
	refs                    []string
	selected                map[int64]bool
	selectedIDs             map[string]int64
	requested               []Burial
	results                 []GraveResult
	auditErr                error
}

func (s *Service) resolvePurgeRefs(run *purgeRun, state *State) {
	run.selected, run.selectedIDs = map[int64]bool{}, map[string]int64{}
	for _, ref := range run.refs {
		idx, err := s.findBurial(state, ref)
		if err == nil && idx < 0 {
			err = missingBurial(ref)
		}
		if err != nil {
			run.results = append(run.results, GraveResult{Ref: ref, Error: err.Error()})
			continue
		}
		run.selected[state.Burials[idx].ID] = true
		run.selectedIDs[ref] = state.Burials[idx].ID
	}
}

func (s *Service) purgeDue(b Burial, run *purgeRun) bool {
	if run.refs != nil {
		return run.selected[b.ID]
	}
	return run.all || (b.PurgeIntent != nil || !b.PurgeAt.After(s.now())) && (!run.background || !b.NextPurgeAttempt.After(s.now()))
}

func (s *Service) purgeLocked(run *purgeRun) error {
	state, err := s.Store.Read()
	if err != nil {
		return err
	}
	s.resolvePurgeRefs(run, &state)
	due := func(b Burial) bool {
		return s.purgeDue(b, run) || run.refs == nil && containerIntent(&state, b.ID) != nil
	}
	if !slices.ContainsFunc(state.Burials, due) {
		return nil
	}
	for _, b := range state.Burials {
		if due(b) {
			run.requested = append(run.requested, b)
		}
	}
	// Enumeration and deletion share the same lock boundary. A failed root
	// scan protects every burial on that root, including explicit purge.
	pending, _ := s.scanManifests(state)
	blocked := s.blockedPurges(state, pending)
	if run.background && !run.dryRun {
		files, err := s.purgeContainerRequests(run, state)
		if err != nil {
			return err
		}
		rows, err := s.purgeExpiredBatches(state, files, blocked)
		run.results = append(run.results, rows...)
		return err
	}
	for _, b := range state.Burials {
		if due(b) {
			if err := s.purgeRequestedBurial(run, state, b, pending, blocked[b.ID]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) blockedPurges(state State, pending []PendingGrave) map[int64]string {
	blocked := map[int64]string{}
	for _, p := range pending {
		if p.Status != "unverifiable" {
			continue
		}
		for _, id := range s.manifestIDs(state, p) {
			blocked[id] = p.Error
		}
	}
	return blocked
}

func (s *Service) purgeExplicitRestore(run *purgeRun, b Burial, pending []PendingGrave) (bool, error) {
	if !run.selected[b.ID] {
		return false, nil
	}
	for _, p := range pending {
		if p.restore == nil || p.Burial == nil || p.Burial.ID != b.ID {
			continue
		}
		r, err := s.purgeRestoreIntent(b, p, run.dryRun)
		if commitFailed(err) {
			r.Error = err.Error()
		}
		run.results = append(run.results, r)
		if commitFailed(err) {
			return true, err
		}
		run.auditErr = errors.Join(run.auditErr, err)
		return true, nil
	}
	return false, nil
}

func (s *Service) purgeRequestedBurial(run *purgeRun, state State, b Burial, pending []PendingGrave, blocked string) error {
	if handled, err := s.purgeExplicitRestore(run, b, pending); handled {
		return err
	}
	if blocked != "" {
		r, err := s.blockedPurge(b, blocked, run.dryRun)
		run.results = append(run.results, r)
		return err
	}
	r := GraveResult{Ref: fmt.Sprintf("#%d", b.ID), Burial: &b}
	err := s.purgeBurial(&r, state, run.selected[b.ID], run.all, run.dryRun)
	if commitFailed(err) {
		r.Error = err.Error()
		run.results = append(run.results, r)
		return err
	}
	run.auditErr = errors.Join(run.auditErr, err)
	err = s.retryPurgeResult(&r, run.dryRun)
	if err != nil || r.Action != "" || r.Error != "" {
		run.results = append(run.results, r)
	}
	return err
}

func (s *Service) blockedPurge(b Burial, detail string, dryRun bool) (GraveResult, error) {
	if !dryRun {
		if err := s.retryPurge(&b, detail); err != nil {
			return GraveResult{Ref: fmt.Sprint(b.ID), Burial: &b, Action: ActionBlocked, Error: errors.Join(errors.New(detail), err).Error()}, err
		}
	}
	if !b.NextPurgeAttempt.IsZero() {
		detail += "; next retry " + b.NextPurgeAttempt.Format(time.RFC3339)
	}
	return GraveResult{Ref: fmt.Sprintf("#%d", b.ID), Action: ActionBlocked, Burial: &b, Error: detail}, nil
}

func (s *Service) retryPurgeResult(r *GraveResult, dryRun bool) error {
	if r.Error == "" {
		return nil
	}
	if dryRun {
		r.Action = ActionBlocked
		return nil
	}
	err := s.retryPurge(r.Burial, r.Error)
	if err != nil {
		r.Error = errors.Join(errors.New(r.Error), err).Error()
	}
	return err
}

func (run *purgeRun) receipts(err error) []GraveResult {
	if run.refs != nil {
		return run.referencedReceipts()
	}
	if err != nil {
		for _, b := range run.requested {
			if !slices.ContainsFunc(run.results, func(r GraveResult) bool { return r.Burial != nil && r.Burial.ID == b.ID }) {
				run.results = append(run.results, GraveResult{Ref: fmt.Sprint(b.ID), Burial: &b, Action: ActionNotExecuted, Error: "not executed: an earlier operation failed"})
			}
		}
	}
	return run.results
}

func (run *purgeRun) referencedReceipts() []GraveResult {
	byID, byRef := make(map[int64]GraveResult), make(map[string]GraveResult)
	for _, r := range run.results {
		if r.Burial != nil {
			byID[r.Burial.ID] = r
		} else {
			byRef[r.Ref] = r
		}
	}
	results := make([]GraveResult, len(run.refs))
	for i, ref := range run.refs {
		r, ok := byID[run.selectedIDs[ref]]
		if !ok {
			r, ok = byRef[ref]
		}
		if !ok {
			r = GraveResult{Action: ActionNotExecuted, Error: "not executed: an earlier operation failed"}
		}
		r.Ref = ref
		results[i] = r
	}
	return results
}
