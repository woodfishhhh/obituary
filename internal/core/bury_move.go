package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// An injected interruption leaves recovery entirely to persisted evidence.
func (s *Service) burialStep(step string) error {
	if s.afterBurialStep != nil {
		return s.afterBurialStep(step)
	}
	return nil
}

type preparedBurial struct {
	out          Outcome
	burial       Burial
	manifest     string
	moved        bool
	retained     bool // an unverified directory stays quarantined with its manifest
	watch        *WatchSnapshot
	sourceParent *os.Root
	owner        func(context.Context, string) Ownership
}

func (s *Service) prepareBurial(out Outcome, reason string) *preparedBurial {
	p := &preparedBurial{out: out}
	if err := s.burialStep("reserved"); err != nil {
		p.out = ledgerOutcome(out, err)
		return p
	}
	now := s.now()
	path, longNames, err := s.FS.LongPath(out.Path)
	if err != nil {
		p.out = failedMove(out, err)
		return p
	}
	out.Path = path
	parents, err := s.captureParents(out.Path)
	if err != nil {
		p.out = failedMove(out, err)
		return p
	}
	grave, graveID, err := s.prepareGrave(out.Path, out.Item.ID, now)
	if err != nil {
		p.out = failedMove(out, err)
		return p
	}
	if err := s.burialStep("prepared"); err != nil {
		p.out = ledgerOutcome(out, err)
		return p
	}
	b := Burial{ID: out.Item.ID, Path: out.Path, Grave: grave, GraveID: graveID,
		Parents: parents, LongNames: longNames,
		Identity: out.Item.Identity, IsDir: out.Item.IsDir, BuriedAt: now, PurgeAt: now.Add(s.retention()),
		Reason: reason, Note: out.Item.Note, Source: out.Item.Source, Project: out.Item.Project, Session: out.Item.Session, WatchID: out.Item.WatchID}
	if reason == ReasonBury {
		b.Operation = s.burialOperation
	}
	manifest := filepath.Dir(grave) + ".json"
	if err := writeManifest(manifest, b); err != nil {
		_ = os.Remove(manifest + ".tmp") // only this attempt's metadata
		p.out = failedMove(out, errors.Join(err, s.discardEmptyGrave(manifest, b)))
		return p
	}
	p.out, p.burial, p.manifest = out, b, manifest
	if err := s.burialStep("manifest"); err != nil {
		p.out = ledgerOutcome(out, err)
	}
	return p
}

// Caller owns store.lock and graveyard.lock. Complete all potentially slow
// validation, including child manifests and rules, before the final Git query.
func (s *Service) validatePrepared(st *State, p *preparedBurial, reason string) (string, error) {
	out, b := &p.out, &p.burial
	if err := st.requireAuditSpace(); err != nil {
		return "", err
	}
	if _, err := s.burialRevision(st, out.Item, reason); err != nil {
		*out = ledgerOutcome(*out, err)
		return "", out.err
	}
	if s.approvedWatch != nil {
		if err := s.checkWatchBurialReview(st, *s.approvedWatch); err != nil {
			*out = failedMove(*out, err)
			if errors.Is(err, ErrProposalChanged) {
				out.Action = ActionSkipped
			}
			return "", nil
		}
	}
	if st.BurialIndex(b.ID) >= 0 || st.ProposalIndex(b.ID) >= 0 {
		return "", fmt.Errorf("#%d is already in use", b.ID)
	}
	children, err := s.childLifespans(st, out.Item, out.Path, reason)
	if err != nil {
		*out = failedMove(*out, err)
		return "", nil
	}
	b.Children, b.Contains = nil, nil
	for _, child := range children {
		b.Children = append(b.Children, BuriedChild{ID: child.ID, Identity: child.Identity, WatchID: child.WatchID})
		b.Contains = append(b.Contains, child.ID)
	}
	if len(children) > 0 {
		// Publish this transaction's children before moving their parent.
		if err := writeManifest(p.manifest, *b); err != nil {
			*out = failedMove(*out, err)
			return "", nil
		}
	}
	// Refuse an unrepresentable recovery record before a directory scan can
	// consume the guard budget. The guard still precedes every move.
	guardCtx, cancelGuard := context.WithTimeout(s.Store.Context(), sweepBatchWork)
	guardErr := s.Guard.CheckContext(guardCtx, out.Path, out.Item.Force)
	cancelGuard()
	if err := guardErr; err != nil {
		*out = failedMove(*out, err)
		return "", nil
	}
	if reason == ReasonExpired && automatic(out.Item) || s.approvedWatch != nil {
		real, action, detail := s.recheckAutomaticRules(out.Item, out.Path)
		if action != "" {
			out.Action, out.Detail = action, detail
		}
		return real, nil
	}
	return "", nil
}

func (s *Service) movePrepared(st *State, p *preparedBurial, reason string) error {
	out, b := &p.out, &p.burial
	real, err := s.validatePrepared(st, p, reason)
	if err != nil || out.Action != "" {
		return err
	}
	release, err := s.holdWatchForMove(st, out.Item, out.Path, reason)
	if err != nil {
		*out = failedMove(*out, err)
		return nil
	}
	defer release()
	defer func() {
		if p.sourceParent != nil {
			p.sourceParent.Close()
			p.sourceParent = nil
		}
	}()
	if err := s.prepareWatchVerification(st, p, reason); err != nil {
		*out = failedMove(*out, err)
		return nil
	}
	if s.watchMove(out.Item, reason) {
		if err := p.checkWatchOriginalDownload(); err != nil {
			*out = failedMove(*out, err)
			return nil
		}
	}
	if reason == ReasonExpired && automatic(out.Item) || s.approvedWatch != nil {
		action, detail, err := s.finalGitForMove(out.Item, real, p.owner)
		if err != nil {
			return err
		}
		if action != "" {
			out.Action, out.Detail = action, detail
			return nil
		}
	}
	if err := s.FS.MovePinned(out.Path, *out.Item.Identity, b.Grave); err != nil {
		*out = failedMove(*out, err)
		return nil
	}
	if !s.verifyWatchMovement(st, p) {
		return nil
	}
	p.moved = true
	out.Grave, out.GraveID = b.Grave, b.GraveID
	if err := s.burialStep("moved"); err != nil {
		return err
	}
	return s.entomb(st, b)
}

// Finalize only after the complete transaction is durably confirmed. Failed
// and unconfirmed commits retain every moved object's independent manifest.
func (s *Service) finishBurial(p *preparedBurial, commitErr error) error {
	if p.retained {
		return commitErr
	}
	if !p.moved {
		if p.manifest != "" {
			return errors.Join(commitErr, s.discardEmptyGrave(p.manifest, p.burial))
		}
		return commitErr
	}
	if commitFailed(commitErr) {
		p.out = ledgerOutcome(p.out, commitErr)
		return commitErr
	}
	p.out.Action, p.out.Burial = ActionBuried, &p.burial
	if err := s.burialStep("committed"); err != nil {
		return err
	}
	if err := os.Remove(p.manifest); err != nil {
		return err
	}
	return commitErr
}

func (s *Service) moveToGrave(out Outcome, reason string) Outcome {
	p := s.prepareBurial(out, reason)
	if p.out.Action != "" || p.out.err != nil {
		return p.out
	}
	var owners *ownershipBatch
	if reason == ReasonExpired && automatic(out.Item) || s.approvedWatch != nil {
		// Folder policies also use this single-item path in recycle mode.
		item := out.Item
		item.Path = out.Path
		owners = s.prepareBatchOwners([]Item{item}, nil)
	}
	err := s.commitPreparedBurial(p, reason, owners)
	if commitFailed(err) || !p.moved {
		return ledgerOutcome(p.out, err)
	}
	return s.measureBurial(p, err)
}

func (s *Service) commitPreparedBurial(p *preparedBurial, reason string, owners *ownershipBatch) error {
	for {
		if owners != nil {
			owners.prefetch(s.Store.Context(), []*preparedBurial{p})
			p.owner = owners.lookup
			s.gitMoveBudget = gitVerificationBudget(owners.elapsed)
		}
		err := s.withGraveyardLock(func() error {
			commitErr := s.Store.UpdateGraveyard(func(st *State) error { return s.movePrepared(st, p, reason) })
			if errors.Is(commitErr, errOwnerSnapshotRefresh) {
				return commitErr // Keep the prepared grave for the next transaction.
			}
			return s.finishBurial(p, commitErr)
		})
		if !errors.Is(err, errOwnerSnapshotRefresh) {
			return err
		}
	}
}

// Size accounting follows committed movement and manifest cleanup, outside
// both locks. Preserve the interruption points around the potentially long walk.
func (s *Service) measureBurial(p *preparedBurial, commitErr error) Outcome {
	if stop := s.burialStep("cleaned"); stop != nil {
		return ledgerOutcome(p.out, stop)
	}
	// Large walks never hold store.lock or graveyard.lock.
	measured := p.burial
	measured.Size = treeSize(measured.Grave)
	if stop := s.burialStep("measured"); stop != nil {
		return ledgerOutcome(p.out, stop)
	}
	sizeErr := s.saveBurialSizes([]Burial{measured})
	if commitFailed(sizeErr) {
		return ledgerOutcome(p.out, sizeErr)
	}
	p.out.Size, p.out.Burial = measured.Size, &measured
	return ledgerOutcome(p.out, errors.Join(commitErr, sizeErr))
}
