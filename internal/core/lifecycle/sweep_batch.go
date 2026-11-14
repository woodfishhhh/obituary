package lifecycle

import (
	"context"
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
		finalErr = s.saveSweepBurialSizes(outcomes, finalErr)
	}()
	var pending []*preparedBurial
	current, refresh := state, false
	flush := func() error {
		rows, err := s.commitBurialBatch(pending, owners, ReasonExpired)
		outcomes = append(outcomes, rows...)
		pending = nil
		refresh = true
		return err
	}
	for _, snapshot := range state.Items {
		if blocked[snapshot.ID] || !snapshot.Due(s.now()) || awaitingWatchBurial(&state, snapshot) {
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
		if idx < 0 || !current.Items[idx].Due(s.now()) || awaitingWatchBurial(&current, current.Items[idx]) {
			continue
		}
		item := current.Items[idx]
		p := s.prepareExpiredBurial(item, roots, &current)
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

func (s *Service) prepareExpiredBurial(item Item, roots []resolvedGraveyardRoot, current *State) *preparedBurial {
	out := s.prepareSettlement(item, false, ReasonExpired, roots, current)
	if out.Action == "" && out.err == nil && item.IsDir && !item.Force {
		// Search the folder for a repository here, without store.lock and
		// with the time a large folder needs.
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
		p = s.prepareBurial(out, ReasonExpired)
	}
	return p
}

func (s *Service) saveSweepBurialSizes(outcomes []Outcome, finalErr error) error {
	var measured []Burial
	for _, out := range outcomes {
		if out.Action == ActionBuried && out.Burial != nil {
			measured = append(measured, *out.Burial)
		}
	}
	if len(measured) > 0 {
		finalErr = errors.Join(finalErr, s.saveBurialSizes(measured))
	}
	return finalErr
}

func (s *Service) commitBurialBatch(pending []*preparedBurial, owners *ownershipBatch, reason string) (outcomes []Outcome, finalErr error) {
	requestContext := s.Store.Context()
	if reason == ReasonBury {
		s = s.WithContext(context.WithoutCancel(requestContext))
	}
	if owners != nil {
		for _, p := range pending {
			p.owner = owners.lookup
		}
	}
	for len(pending) > 0 {
		if owners != nil {
			// Git queries run outside store.lock; Current revalidates each snapshot next to its move.
			owners.prefetch(s.Store.Context(), pending)
			s.gitMoveBudget = gitVerificationBudget(owners.elapsed)
		}
		processed, auditCount, stopErr, err := s.commitBurialPrefix(pending, owners, reason, requestContext, len(outcomes) > 0)
		for _, p := range pending[:processed] {
			outcomes = append(outcomes, p.out)
		}
		// A cleanup error does not undo the confirmed transaction.
		// Explicit interruptions leave accounting to recovery with its receipt.
		if reason == ReasonExpired || !commitFailed(err) {
			if measureErr := s.measureBurialBatch(pending[:processed], outcomes[len(outcomes)-processed:]); measureErr != nil {
				return outcomes, errors.Join(err, measureErr)
			}
		}
		if commitFailed(err) {
			return outcomes, err
		}
		finalErr = errors.Join(finalErr, err)
		pending = pending[processed:]
		if stopErr != nil {
			return outcomes, errors.Join(finalErr, s.discardUnmovedBatch(pending, stopErr))
		}
		if auditCount >= maxPendingAudit-sweepBatchSize {
			if err := s.Store.FlushAudit(); err != nil {
				return outcomes, errors.Join(finalErr, err)
			}
		}
	}
	return outcomes, finalErr
}

func (s *Service) commitBurialPrefix(pending []*preparedBurial, owners *ownershipBatch, reason string, ctx context.Context, started bool) (processed, auditCount int, stopErr, err error) {
	err = s.withGraveyardLock(func() error {
		commitErr := s.Store.UpdateGraveyard(func(st *State) error {
			var moveErr error
			processed, stopErr, moveErr = s.moveBurialPrefix(st, pending, owners, reason, ctx, started)
			auditCount = len(st.PendingAudit)
			return moveErr
		})
		return s.finishBurialBatch(pending[:processed], commitErr)
	})
	return
}

func (s *Service) moveBurialPrefix(st *State, pending []*preparedBurial, owners *ownershipBatch, reason string, ctx context.Context, alreadyStarted bool) (processed int, stopErr, err error) {
	started := time.Now()
	for _, p := range pending {
		if reason == ReasonBury && (processed > 0 || alreadyStarted) && ctx.Err() != nil {
			return processed, ctx.Err(), nil
		}
		// An in-flight syscall cannot be preempted; yield before starting another item.
		if processed > 0 && time.Since(started) >= sweepBatchWork {
			break
		}
		if err := requireAuditSpace(st); err != nil {
			return processed, err, nil
		}
		if p.out.Action == "" {
			err := s.movePrepared(st, p, reason)
			if errors.Is(err, errOwnerSnapshotRefresh) {
				break
			}
			processed++
			if err != nil {
				return processed, nil, err
			}
		} else {
			processed++
		}
		if owners != nil {
			owners.done[p.out.Item.ID] = true
		}
		if !p.moved {
			if reason == ReasonExpired {
				s.commitOutcome(st, p.out)
			}
			queueAudit(st, Event{Time: s.now(), Action: p.out.Action, ID: p.out.Item.ID, Path: p.out.Path, Detail: p.out.Detail})
		}
	}
	return processed, nil, nil
}

func (s *Service) finishBurialBatch(pending []*preparedBurial, commitErr error) error {
	if !commitFailed(commitErr) {
		for _, p := range pending {
			if p.moved {
				p.out.Action, p.out.Burial = ActionBuried, &p.burial
			}
		}
	}
	for _, p := range pending {
		finishErr := s.finishBurial(p, commitErr)
		if commitFailed(commitErr) {
			p.out = ledgerOutcome(p.out, commitErr)
			continue // Every moved receipt retains its independent manifest.
		}
		if commitFailed(finishErr) {
			return finishErr
		}
	}
	return commitErr
}

func (s *Service) measureBurialBatch(pending []*preparedBurial, outcomes []Outcome) error {
	// Confirmed moves remain measurable even if an earlier manifest cleanup failed.
	for i, p := range pending {
		if p.out.Burial == nil {
			continue
		}
		if err := s.burialStep("cleaned"); err != nil {
			return err
		}
		p.burial.Size = treeSize(p.burial.Grave)
		if err := s.burialStep("measured"); err != nil {
			return err
		}
		outcomes[i].Size = p.burial.Size
	}
	return nil
}

func (s *Service) discardUnmovedBatch(pending []*preparedBurial, stopErr error) error {
	for _, p := range pending {
		if p.manifest != "" {
			stopErr = errors.Join(stopErr, s.discardEmptyGrave(p.manifest, p.burial))
		}
	}
	return stopErr
}
