package lifecycle

import (
	"context"
	"errors"
	"fmt"
)

type trackedBurialEntry struct {
	index int
	ref   string
	item  Item
}

// BuryTracked moves explicitly referenced tracked items into the graveyard
// right away (or the Recycle Bin in recycle mode). Only currently tracked
// items are accepted; untracked paths and already-buried IDs are refused in
// their result slots without touching unrelated files.
func (s *Service) BuryTracked(refs []string) (results []GraveResult, err error) {
	s, notify := s.trackChanges()
	defer notify()
	if s.burialOperation == nil {
		s, err = s.BeginBurial("", "")
		if err != nil {
			return nil, err
		}
	}
	s = s.WithContext(s.Store.Context())
	s, flushAudit := s.auditBatch(false)
	defer func() { err = errors.Join(err, flushAudit()) }()
	s.confirmedOwners = make(map[string]bool)
	return s.buryTrackedRefs(refs)
}

func (s *Service) buryTrackedRefs(refs []string) ([]GraveResult, error) {
	results := make([]GraveResult, len(refs))
	for i, ref := range refs {
		results[i] = GraveResult{Ref: ref, Action: ActionNotExecuted, Error: "not executed: an earlier operation failed"}
	}
	release, err := s.waitSweep()
	if err != nil {
		for i := range results {
			results[i].Error = "not executed: " + err.Error()
		}
		return results, err
	}
	defer release()
	recovery, err := s.recoverManifests()
	if commitFailed(err) {
		return results, err
	}
	auditErr := err
	state, err := s.Store.Read()
	if commitFailed(err) {
		return results, err
	}
	auditErr = errors.Join(auditErr, err)
	roots := s.resolveMissingGraveyardRoots(recovery.locations, s.graveyardRoots(state))
	entries := s.resolveTrackedBurials(refs, &state, recovery, results)
	return results, errors.Join(auditErr, s.executeTrackedBurials(entries, state, roots, results))
}

func (s *Service) resolveTrackedBurials(refs []string, st *State, recovery recovery, results []GraveResult) []trackedBurialEntry {
	seen := make(map[int64]bool, len(refs))
	entries := make([]trackedBurialEntry, 0, len(refs))
	for i, ref := range refs {
		item, row := s.resolveTrackedRef(st, ref, seen, recovery)
		if item == nil {
			results[i] = row
			continue
		}
		seen[item.ID] = true
		entries = append(entries, trackedBurialEntry{index: i, ref: ref, item: *item})
	}
	return entries
}

func (s *Service) resolveTrackedRef(st *State, ref string, seen map[int64]bool, recovery recovery) (*Item, GraveResult) {
	row := GraveResult{Ref: ref}
	idx, err := s.lookupTrackedItem(st, ref)
	if err != nil {
		row.Error = err.Error()
		return nil, row
	}
	if idx < 0 {
		row.Error = s.untrackedLifespanHint(st, ref).Error()
		return nil, row
	}
	item := st.Items[idx]
	if seen[item.ID] {
		row.Error = fmt.Sprintf("#%d is already in this batch", item.ID)
		return nil, row
	}
	if recovery.blocked[item.ID] {
		row.Action, row.Error = ActionRetry, "unverifiable burial manifest; see obit doctor"
		return nil, row
	}
	if item.Kind == "" {
		if detail := recovery.roots[ProjectKey(s.graveyardRoot(item.Path))]; detail != "" {
			row.Action, row.Error = ActionBlocked, detail
			return nil, row
		}
	}
	return &item, row
}

// lookupTrackedItem never falls back to a cwd-relative file when ref is numeric.
func (s *Service) lookupTrackedItem(st *State, ref string) (int, error) {
	if id, numeric, err := BurialReferenceID(ref); numeric || err != nil {
		if err != nil {
			return -1, err
		}
		return st.Index(id), nil
	}
	idx, _, err := s.find(st, ref)
	return idx, err
}

func (s *Service) executeTrackedBurials(entries []trackedBurialEntry, state State, roots []resolvedGraveyardRoot, results []GraveResult) (finalErr error) {
	var batch []trackedBurialEntry
	current, refresh := state, false
	flush := func() error {
		err := s.flushTrackedBatch(batch, &current, &refresh, roots, results)
		batch = nil
		finalErr = errors.Join(finalErr, err)
		return err
	}
	for _, entry := range entries {
		if err := s.Store.Context().Err(); err != nil {
			return errors.Join(finalErr, flush(), err)
		}
		if entry.item.Kind != "" || s.recycleMode() {
			if commitFailed(flush()) {
				return finalErr
			}
			err := s.burySingleTracked(entry, roots, results)
			finalErr = errors.Join(finalErr, err)
			if commitFailed(err) {
				return finalErr
			}
			refresh = true
			continue
		}
		if len(batch) == sweepBatchSize || s.trackedBurialOverlap(entry.item.Path, batch) {
			if commitFailed(flush()) {
				return finalErr
			}
		}
		batch = append(batch, entry)
	}
	flush()
	return finalErr
}

func (s *Service) trackedBurialOverlap(path string, batch []trackedBurialEntry) bool {
	real, err := s.FS.RealPath(path)
	if err != nil {
		real = path
	}
	for _, entry := range batch {
		previous, err := s.FS.RealPath(entry.item.Path)
		if err != nil {
			previous = entry.item.Path
		}
		if samePath(real, previous) || isInside(real, previous) || isInside(previous, real) {
			return true
		}
	}
	return false
}

func (s *Service) burySingleTracked(entry trackedBurialEntry, roots []resolvedGraveyardRoot, results []GraveResult) error {
	var out Outcome
	switch entry.item.Kind {
	case KindContainer:
		out = s.stopContainer(entry.item, false)
	case KindDockerVolume:
		out = s.settleDockerVolume(entry.item, false, ReasonBury)
	default:
		tx := s.WithContext(context.WithoutCancel(s.Store.Context()))
		out = tx.settle(entry.item, false, ReasonBury, roots)
		res, err := tx.finishBuryRef(entry.ref, entry.item, out, false)
		results[entry.index] = res
		return err
	}
	res, err := s.finishBuryRef(entry.ref, entry.item, out, true)
	results[entry.index] = res
	return err
}

func (s *Service) flushTrackedBatch(batch []trackedBurialEntry, current *State, refresh *bool, roots []resolvedGraveyardRoot, results []GraveResult) error {
	if len(batch) == 0 {
		return nil
	}
	if *refresh {
		state, err := s.Store.Read()
		if commitFailed(err) {
			results[batch[0].index].Error = err.Error()
			return err
		}
		*current, *refresh = state, false
	}
	pending, active, prepErr := s.prepareTrackedBatch(batch, current, roots, results)
	if len(pending) == 0 {
		return prepErr
	}
	*refresh = true
	return errors.Join(s.commitTrackedBatch(pending, active, results), prepErr)
}

func (s *Service) prepareTrackedBatch(batch []trackedBurialEntry, current *State, roots []resolvedGraveyardRoot, results []GraveResult) ([]*preparedBurial, []trackedBurialEntry, error) {
	pending := make([]*preparedBurial, 0, len(batch))
	active := make([]trackedBurialEntry, 0, len(batch))
	for _, entry := range batch {
		idx := current.Index(entry.item.ID)
		if idx < 0 {
			results[entry.index] = GraveResult{Ref: entry.ref, Error: s.untrackedLifespanHint(current, entry.ref).Error()}
			continue
		}
		entry.item = current.Items[idx]
		p := s.prepareTrackedBurial(entry.item, roots, current)
		if p.out.err != nil {
			res, err := s.finishBuryRef(entry.ref, entry.item, p.out, true)
			results[entry.index] = res
			return pending, active, err
		}
		pending = append(pending, p)
		active = append(active, entry)
	}
	return pending, active, nil
}

func (s *Service) prepareTrackedBurial(item Item, roots []resolvedGraveyardRoot, current *State) *preparedBurial {
	out := s.prepareSettlement(item, false, ReasonBury, roots, current)
	if out.Action == "" && out.err == nil && item.IsDir && !item.Force {
		ctx, cancel := context.WithTimeout(s.Store.Context(), guardWalkBudget)
		err := s.Guard.CheckContext(ctx, out.Path, false)
		cancel()
		if err != nil {
			return &preparedBurial{out: settlementGuardFailure(out, err)}
		}
		out.searched = true
	}
	p := &preparedBurial{out: out}
	if out.Action == "" && out.err == nil {
		p = s.prepareBurial(out, ReasonBury)
	}
	return p
}

func (s *Service) commitTrackedBatch(pending []*preparedBurial, active []trackedBurialEntry, results []GraveResult) error {
	rows, err := s.commitBurialBatch(pending, nil, ReasonBury)
	var measured []Burial
	for i, out := range rows {
		entry := active[i]
		results[entry.index], _ = s.finishBuryRef(entry.ref, entry.item, out, true)
		if out.Burial != nil {
			measured = append(measured, *out.Burial)
		}
	}
	if len(measured) > 0 {
		tx := s.WithContext(context.WithoutCancel(s.Store.Context()))
		err = errors.Join(err, tx.saveBurialSizes(measured))
	}
	return err
}
