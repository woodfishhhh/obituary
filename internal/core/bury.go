package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// entomb records one manifest's burial, retaining its original timestamps.
func (s *Service) entomb(st *State, b *Burial) error {
	// Recovery also enters here, after another operation may have filled the
	// outbox. Keep the manifest until its event fits in the same transaction.
	if err := st.requireAuditSpace(); err != nil {
		return err
	}
	if st.BurialIndex(b.ID) >= 0 || st.ProposalIndex(b.ID) >= 0 {
		return fmt.Errorf("#%d is already in use; leaving its grave unchanged", b.ID)
	}
	if err := s.reconcileChildren(st, b); err != nil {
		return err
	}
	st.Burials = append(st.Burials, *b)
	st.NextID = max(st.NextID, b.ID+1)
	st.Stats.Buried++
	if b.Reason == ReasonBury && b.Source == "delete-command" {
		st.Stats.DeleteBuried++
	}
	st.Stats.BuriedBytes += b.Size
	st.queueAudit(Event{Time: s.now(), Action: ActionBuried, ID: b.ID, Path: b.Path, Detail: b.Grave})
	st.dropWatchBurial(b.ID)
	for _, child := range b.Children {
		st.dropWatchBurial(child.ID)
	}
	st.Items = slices.DeleteFunc(st.Items, func(it Item) bool {
		return it.ID == b.ID || slices.ContainsFunc(b.Children, func(child BuriedChild) bool { return it.ID == child.ID })
	})
	return nil
}

// Bury moves paths into the graveyard right away (or to the Recycle Bin in
// recycle mode). A tracked path keeps its number and stops being tracked.
func (s *Service) Bury(paths []string, force bool, note string) ([]GraveResult, error) {
	return s.bury(paths, force, note, "")
}

// BuryDeletion records the explicit command source in the manifest, so normal
// commits and recovery account for the same burial in the ledger transaction.
func (s *Service) BuryDeletion(paths []string, force bool, command string) ([]GraveResult, error) {
	return s.bury(paths, force, command, "delete-command")
}

// BuryDeletionChecked applies command-specific checks in operand order, after
// earlier overlapping paths finish. A false check silently skips that operand.
// The callback runs outside ledger locks and must not move the object.
func (s *Service) BuryDeletionChecked(paths []string, force bool, command string, check func(string) (bool, error)) ([]GraveResult, error) {
	copy := *s
	copy.deletionCheck = check
	return copy.BuryDeletion(paths, force, command)
}

// bury aborts on setup, recovery or ledger commit failures, retaining completed
// and unexecuted receipts. Per-path refusals stay in their slots; audit failures
// are accumulated while the remaining paths run.
func (s *Service) bury(paths []string, force bool, note, source string) (results []GraveResult, err error) {
	s, notify := s.trackChanges()
	defer notify()
	if s.burialOperation == nil {
		s, err = s.BeginBurial("", "")
		if err != nil {
			return nil, err
		}
	}
	s = s.WithContext(s.Store.Context())
	s.Store.auditLines = new(int)
	defer func() { results = limitBuryRecovery(results, len(paths), *s.Store.auditLines) }()
	s, flushAudit := s.auditBatch(false)
	defer func() { err = errors.Join(err, flushAudit()) }()
	s.confirmedOwners = make(map[string]bool)
	if err := ValidateMetadata("note", note); err != nil {
		return nil, err
	}
	return s.buryPaths(paths, force, note, source)
}

func (s *Service) buryPaths(paths []string, force bool, note, source string) ([]GraveResult, error) {
	results := make([]GraveResult, len(paths))
	for i, ref := range paths {
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
	recovery, err := s.recoverManifests(paths...)
	if commitFailed(err) {
		return s.failedBuryRecovery(paths, results, recovery), err
	}
	auditErr := err
	for _, o := range recovery.outcomes {
		if o.Action == ActionResurrected {
			results = append(results, GraveResult{Ref: fmt.Sprintf("#%d", o.Item.ID), Action: o.Action, Burial: o.Burial, Path: o.Path, Warning: o.Detail})
		}
	}
	if !s.recycleMode() {
		return results, errors.Join(auditErr, s.buryBatches(paths, results, force, note, source, recovery))
	}
	for i, ref := range paths {
		if s.deletionCheck != nil {
			ok, checkErr := s.deletionCheck(ref)
			if !ok || checkErr != nil {
				results[i] = GraveResult{Ref: ref}
				if checkErr != nil {
					results[i].Error = checkErr.Error()
				}
				continue
			}
		}
		results[i], err = s.buryRef(ref, force, note, source, recovery)
		if commitFailed(err) {
			return results, err
		}
		auditErr = errors.Join(auditErr, err)
	}
	return results, auditErr
}

// Recovery may already have confirmed and retired the only manifest. Deliver
// those numbered receipts even if later accounting failed.
func (s *Service) failedBuryRecovery(paths []string, results []GraveResult, recovery recovery) []GraveResult {
	for _, o := range recovery.outcomes {
		if o.Burial == nil || o.Action != ActionBuried && o.Action != ActionResurrected {
			continue
		}
		r := GraveResult{Ref: fmt.Sprintf("#%d", o.Burial.ID), Action: o.Action, Burial: o.Burial, Path: o.Path, Warning: o.Detail}
		matched := false
		for i, ref := range paths {
			if s.matchesRecoveredBurial(ref, *o.Burial, recovery.burials) {
				results[i], matched = r, true
				break
			}
		}
		if !matched {
			results = append(results, r)
		}
	}
	return results
}

// buryRef keeps inspection/refusal failures in its receipt. Only ledger
// failures abort the caller's batch; audit errors allow later refs to run.
func (s *Service) buryRef(ref string, force bool, note, source string, recovery recovery) (GraveResult, error) {
	r := GraveResult{Ref: ref}
	abs, err := pathutil.Abs(ref)
	if err != nil {
		r.Error = err.Error()
		return r, nil
	}
	if detail := recovery.roots[ProjectKey(s.graveyardRoot(abs))]; detail != "" {
		r.Action, r.Error = ActionBlocked, detail
		return r, nil
	}
	c, err := s.inspect(ref, force)
	if err != nil {
		return s.recoveredBuryRef(ref, abs, recovery.burials, err), nil
	}
	item, err := s.reserveBuryItem(c)
	if commitFailed(err) {
		r.Action, r.Error = ActionNotExecuted, err.Error()
		return r, err
	}
	if recovery.blocked[item.ID] {
		r.Action, r.Error = ActionRetry, "unverifiable burial manifest; see obit doctor"
		return r, err
	}
	identity := c.identity
	item.Path, item.IsDir, item.Identity, item.Force = c.path, c.isDir, &identity, force
	if note != "" {
		item.Note = note
	}
	if source != "" {
		item.Source = source
	}
	// Finish this reserved item's move/ledger protocol even if the client
	// cancels. The next item still uses the request context.
	tx := s.WithContext(context.WithoutCancel(s.Store.Context()))
	o := tx.settle(item, false, ReasonBury, recovery.locations)
	r, finishErr := tx.finishBuryRef(ref, item, o, false)
	if commitFailed(finishErr) {
		return r, finishErr
	}
	return r, errors.Join(err, finishErr)
}

func (s *Service) reserveBuryItem(c *candidate) (Item, error) {
	var item Item
	err := s.Store.UpdateGraveyard(func(st *State) error {
		idx := st.FindIdentity(c.identity)
		if idx >= 0 {
			item = st.Items[idx]
		} else {
			item = Item{ID: st.NextID, CreatedAt: s.now(), ExpiresAt: s.now(), Source: "cli"}
			st.NextID++ // reserving an ID is the only persisted pre-move change
		}
		return nil
	})
	return item, err
}

// Retrying the original explicit request acknowledges a recovery; a new
// object at the same path still receives its own burial.
func (s *Service) recoveredBuryRef(ref, abs string, burials []Burial, err error) GraveResult {
	r := GraveResult{Ref: ref}
	if !errors.Is(err, ErrNotFound) {
		r.Error = err.Error()
		return r
	}
	st := State{Burials: burials}
	expanded, lookupErr := s.referencePath(abs)
	idx := -1
	if lookupErr == nil {
		idx, lookupErr = st.FindBurial(expanded)
	}
	if lookupErr != nil {
		err = lookupErr
	}
	if idx >= 0 {
		r.Action, r.Burial = ActionBuried, &st.Burials[idx]
		return r
	}
	r.Error = err.Error()
	return r
}

// finishBuryRef returns ledger failures to abort the batch, keeps other failed
// outcomes in the receipt, and reports audit failures without stopping it.
func (s *Service) finishBuryRef(ref string, item Item, o Outcome, audited bool) (GraveResult, error) {
	r := GraveResult{Ref: ref, Action: o.Action, Burial: o.Burial}
	if commitFailed(o.err) {
		r.Error = o.err.Error()
		if o.Grave != "" && o.Burial == nil {
			r.Action = ActionRecoveryPending
			r.Grave, r.Detail = o.Grave, o.Detail
		}
		return r, o.err
	}
	auditErr := o.err
	if o.Action == ActionRecycled {
		err := s.commit(o)
		if commitFailed(err) {
			r.Error = err.Error()
			return r, err
		}
		auditErr = errors.Join(auditErr, err)
	} else if o.Action != ActionBuried {
		r.Error = o.Detail
		if r.Error == "" {
			r.Error = "object was not buried: " + o.Action
		}
	}
	if o.Action != ActionBuried && !audited {
		auditErr = errors.Join(auditErr, s.Store.RecordCommitted(Event{Time: s.now(), Action: o.Action, ID: item.ID, Path: item.Path, Detail: o.Detail}))
	}
	return r, auditErr
}

// Input receipts are always the initial prefix. Bound only unrelated recovery
// receipts, for both CLI JSON and MCP, including partial-failure returns.
func limitBuryRecovery(results []GraveResult, inputs, auditLines int) []GraveResult {
	const recoveryLimit = 20
	if len(results) <= inputs {
		return results
	}
	count := len(results) - inputs
	results[0].RecoveryCount = count
	if count > recoveryLimit {
		results[0].RecoveryOmitted = count - recoveryLimit
		results[0].Warning = fmt.Sprintf("%d additional recovery receipts omitted; use obit log --lines %d to inspect recovered operations. %s", count-recoveryLimit, count+inputs+auditLines, results[0].Warning)
		results = results[:inputs+recoveryLimit]
	}
	return results
}

func (s *Service) matchesRecoveredBurial(ref string, b Burial, burials []Burial) bool {
	expanded, err := s.referencePath(ref)
	if err != nil {
		return false
	}
	st := State{Burials: burials}
	idx, err := st.FindBurial(expanded)
	if err != nil || idx < 0 || !sameBurial(st.Burials[idx], b) {
		return false
	}
	id, err := s.FS.Identify(expanded)
	return errors.Is(err, ErrNotFound) || err == nil && b.Identity != nil && id.Serial == b.Identity.Serial && id.FileID == b.Identity.FileID
}

// Explicit requests wait briefly for background work, while Sweep stays
// nonblocking. Real time bounds the wait even when the service clock is fixed.
func (s *Service) waitSweep() (func(), error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := s.Store.Context().Err(); err != nil {
			return nil, err
		}
		release, err := s.lockSweep()
		if !errors.Is(err, ErrSweepBusy) || !time.Now().Before(deadline) {
			return release, err
		}
		select {
		case <-s.Store.Context().Done():
			return nil, s.Store.Context().Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
