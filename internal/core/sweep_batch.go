package core

import (
	"errors"
	"time"
)

// Publish per-object recovery manifests before retiring the old ledger entries
// so filesystem moves can be reconciled after an interrupted JSON transaction.
// Bound both transaction size and work between durable commits. A filesystem
// syscall/Sync already in flight cannot be preempted; no further item starts
// after this budget. Preparation and size walks run without store.lock.
const sweepBatchSize = 32
const sweepBatchWork = 100 * time.Millisecond

func (s *Service) sweepBurials(state State, roots []resolvedGraveyardRoot, blocked map[int64]bool) (outcomes []Outcome, finalErr error) {
	owners := s.prepareBatchOwners(state.Items, blocked)
	defer func() {
		var measured []Burial
		for _, out := range outcomes {
			if out.Action == ActionBuried && out.Burial != nil {
				measured = append(measured, *out.Burial)
			}
		}
		if len(measured) > 0 {
			finalErr = errors.Join(finalErr, s.saveBurialSizes(measured))
		}
	}()
	var pending []*preparedBurial
	current, refresh := state, false
	flush := func() error {
		rows, err := s.commitBurialBatch(pending, owners)
		outcomes = append(outcomes, rows...)
		pending = nil
		refresh = true
		return err
	}
	for _, snapshot := range state.Items {
		if blocked[snapshot.ID] || !snapshot.Due(s.now()) || state.awaitingWatchBurial(snapshot) {
			continue
		}
		if refresh {
			var err error
			current, err = s.Store.Read()
			if err != nil {
				return outcomes, err
			}
			refresh = false
		}
		idx := current.Index(snapshot.ID)
		if idx < 0 || !current.Items[idx].Due(s.now()) || current.awaitingWatchBurial(current.Items[idx]) {
			continue
		}
		item := current.Items[idx]
		out := s.prepareSettlement(item, false, ReasonExpired, roots, &current)
		p := &preparedBurial{out: out}
		if out.Action == "" && out.err == nil {
			p = s.prepareBurial(out, ReasonExpired)
		}
		pending = append(pending, p)
		if p.out.err != nil {
			// A fault seam models a stopped process: do not discard evidence.
			for _, p := range pending {
				outcomes = append(outcomes, p.out)
			}
			return outcomes, p.out.err
		}
		if len(pending) == sweepBatchSize {
			if err := flush(); err != nil {
				return outcomes, err
			}
		}
	}
	if len(pending) > 0 {
		finalErr = flush()
	}
	return outcomes, finalErr
}

func (s *Service) commitBurialBatch(pending []*preparedBurial, owners *ownershipBatch) (outcomes []Outcome, finalErr error) {
	if owners != nil {
		for _, p := range pending {
			p.owner = owners.lookup
		}
	}
	for len(pending) > 0 {
		if owners != nil {
			// Git interprets the index without holding store.lock. The answer
			// is authority only after Current revalidates it next to each move;
			// providers without snapshots still query again inside the lock.
			owners.prefetch(s.Store.Context(), pending)
			s.gitMoveBudget = gitVerificationBudget(owners.elapsed)
		}
		processed, auditCount := 0, 0
		var stopErr error
		err := s.withGraveyardLock(func() error {
			commitErr := s.Store.Update(func(st *State) error {
				// Allow 100ms of work before yielding between items; a final
				// query in flight has its own bounded, measured budget.
				started := time.Now()
				for _, p := range pending {
					if processed > 0 && time.Since(started) >= sweepBatchWork {
						break
					}
					if err := st.requireAuditSpace(); err != nil {
						stopErr = err
						break
					}
					if p.out.Action == "" {
						err := s.movePrepared(st, p, ReasonExpired)
						if errors.Is(err, errOwnerSnapshotRefresh) {
							break // Commit the moved prefix before yielding for Git.
						}
						processed++
						if err != nil {
							return err
						}
					} else {
						processed++
					}
					if owners != nil {
						owners.done[p.out.Item.ID] = true
					}
					if !p.moved {
						s.commitOutcome(st, p.out)
						st.queueAudit(Event{Time: s.now(), Action: p.out.Action, ID: p.out.Item.ID, Path: p.out.Path, Detail: p.out.Detail})
					}
				}
				auditCount = len(st.PendingAudit)
				return nil
			})
			if !commitFailed(commitErr) {
				for _, p := range pending[:processed] {
					if p.moved {
						p.out.Action, p.out.Burial = ActionBuried, &p.burial
					}
				}
			}
			for _, p := range pending[:processed] {
				finishErr := s.finishBurial(p, commitErr)
				if commitFailed(commitErr) {
					p.out = ledgerOutcome(p.out, commitErr)
					continue // classify every moved receipt, preserving every manifest
				}
				if commitFailed(finishErr) {
					return finishErr
				}
			}
			return commitErr
		})
		for _, p := range pending[:processed] {
			outcomes = append(outcomes, p.out)
		}
		// A cleanup error does not undo the confirmed transaction. Measure
		// every confirmed burial before returning it, even if an earlier
		// manifest could not be removed. Unconfirmed moves stay recovery-only.
		for i, p := range pending[:processed] {
			if p.out.Burial == nil {
				continue
			}
			if stepErr := s.burialStep("cleaned"); stepErr != nil {
				return outcomes, errors.Join(err, stepErr)
			}
			p.burial.Size = treeSize(p.burial.Grave)
			if stepErr := s.burialStep("measured"); stepErr != nil {
				return outcomes, errors.Join(err, stepErr)
			}
			outcomes[len(outcomes)-processed+i].Size = p.burial.Size
		}
		if commitFailed(err) {
			return outcomes, err
		}
		finalErr = errors.Join(finalErr, err)
		pending = pending[processed:]
		if stopErr != nil {
			// Nothing beyond the committed prefix moved. Remove only our own
			// unused metadata so an ordinary lifespan edit remains possible.
			for _, p := range pending {
				if p.manifest != "" {
					stopErr = errors.Join(stopErr, s.discardEmptyGrave(p.manifest, p.burial))
				}
			}
			return outcomes, errors.Join(finalErr, stopErr)
		}
		if auditCount >= maxPendingAudit-sweepBatchSize {
			if err := s.Store.FlushAudit(); err != nil {
				return outcomes, errors.Join(finalErr, err)
			}
		}
	}
	return outcomes, finalErr
}
