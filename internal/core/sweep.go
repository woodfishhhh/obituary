package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Tick is one round of background work: reap due items, then purge expired
// burials. ErrSweepBusy skips the whole round; background callers treat it
// as a normal busy result rather than an error to log.
func (s *Service) Tick() (outcomes []Outcome, purged []GraveResult, err error) {
	s, flushAudit := s.auditBatch(false)
	defer func() { err = errors.Join(err, flushAudit()) }()
	outcomes, err = s.Sweep(false)
	if errors.Is(err, ErrSweepBusy) {
		return outcomes, nil, err
	}
	if err != nil {
		var audit *AuditError
		if !errors.As(err, &audit) {
			return outcomes, nil, err
		}
	}
	purged, perr := s.purge(false, false, true)
	if perr != nil {
		return outcomes, purged, perr
	}
	return outcomes, purged, err
}

// Outcome is what a sweep did, or would do, with one due item.
type Outcome struct {
	Item         Item      `json:"item"`
	Action       string    `json:"action"`
	Path         string    `json:"path"` // where the object was found
	Grave        string    `json:"grave,omitempty"`
	GraveID      *Identity `json:"graveId,omitempty"`
	Size         int64     `json:"size,omitempty"`
	Detail       string    `json:"detail,omitempty"`
	Burial       *Burial   `json:"burial,omitempty"`
	err          error     // a ledger failure, distinct from a retryable filesystem outcome
	retryCommand string    // selected from the pre-move ledger's discoverable roots
}

const maxBackoff = time.Hour

// Sweep recycles every due item. With dryRun it only reports what it would do.
//
// Each item is claimed just before it is processed: an item canceled or
// extended while an earlier one was being recycled is skipped, and results are
// only written back if the record still has the revision that was processed.
// Only one sweep runs per store at a time (ErrSweepBusy otherwise).
func (s *Service) Sweep(dryRun bool) ([]Outcome, error) {
	if !dryRun {
		var notify func()
		s, notify = s.trackChanges()
		defer notify()
	}
	return s.sweep(dryRun, make(map[int64]bool))
}

func (s *Service) sweep(dryRun bool, blocked map[int64]bool) (outcomes []Outcome, finalErr error) {
	// Background work has no cancellable entry contract. A move must always
	// finish its ledger commit, including when a future caller supplies a ctx.
	s = s.WithContext(context.WithoutCancel(s.Store.Context()))
	s, flushAudit := s.auditBatch(dryRun)
	defer func() { finalErr = errors.Join(finalErr, flushAudit()) }()
	s.confirmedOwners = make(map[string]bool)
	if !dryRun {
		release, err := s.lockSweep()
		if err != nil {
			return nil, err
		}
		defer release()
		if err := s.pruneAutomaticExemptions(); err != nil {
			return nil, err
		}
	}
	var auditErr error
	var roots []resolvedGraveyardRoot
	if !dryRun {
		r, err := s.recoverManifests()
		if commitFailed(err) {
			return r.outcomes, err
		}
		for id := range r.blocked {
			blocked[id] = true
		}
		outcomes, auditErr = r.outcomes, err
		roots = r.locations
	}
	state, err := s.Store.Read()
	if commitFailed(err) {
		return outcomes, err
	}
	auditErr = errors.Join(auditErr, err)
	containerRows, containerErr := s.sweepContainers(&state, dryRun)
	outcomes = append(outcomes, containerRows...)
	if containerErr != nil {
		return outcomes, containerErr
	}
	// Recovery may have committed records, and concurrent registrations can
	// add a volume. Resolve only roots not already seen in this operation.
	roots = s.resolveMissingGraveyardRoots(roots, s.graveyardRoots(state))
	if !dryRun && !s.recycleMode() {
		batchOut, err := s.sweepBurials(state, roots, blocked)
		return append(outcomes, batchOut...), errors.Join(auditErr, err)
	}
	for _, snapshot := range state.Items {
		if blocked[snapshot.ID] || !snapshot.Due(s.now()) || awaitingWatchBurial(&state, snapshot) {
			continue
		}
		if dryRun {
			out := s.settle(snapshot, true, ReasonExpired, roots)
			outcomes = append(outcomes, out)
			if out.err != nil {
				return outcomes, out.err
			}
			continue
		}
		item, ok, err := s.claim(snapshot.ID)
		if err != nil {
			return outcomes, err
		}
		if !ok {
			continue
		}
		outcome := s.settle(item, false, ReasonExpired, roots)
		outcomes = append(outcomes, outcome)
		if err := s.commit(outcome); err != nil {
			if commitFailed(err) {
				return outcomes, err
			}
			auditErr = errors.Join(auditErr, err)
		}
		if outcome.Action != ActionBuried {
			auditErr = errors.Join(auditErr, s.Store.RecordCommitted(Event{Time: s.now(), Action: outcome.Action, ID: item.ID, Path: outcome.Path, Detail: outcome.Detail}))
		}
	}
	return outcomes, auditErr
}

func (s *Service) lockSweep() (func(), error) {
	if err := os.MkdirAll(s.Store.Home, 0o700); err != nil {
		return nil, err
	}
	release, ok, err := TryLockExclusive(filepath.Join(s.Store.Home, "sweep.lock"))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrSweepBusy
	}
	return release, nil
}

// claim re-reads the item and returns it only if it is still due.
func (s *Service) claim(id int64) (Item, bool, error) {
	state, err := s.Store.Read()
	if err != nil {
		return Item{}, false, err
	}
	idx := state.Index(id)
	if idx < 0 || !state.Items[idx].Due(s.now()) || awaitingWatchBurial(&state, state.Items[idx]) {
		return Item{}, false, nil
	}
	return state.Items[idx], true, nil
}

// commit writes a sweep outcome back, but only onto the revision it processed.
func (s *Service) commit(o Outcome) error {
	if o.err != nil || o.Action == ActionBuried || o.Action == ActionSkipped || o.Action == ActionProposed {
		return o.err // burial was committed in the same transaction as its move
	}
	return s.Store.UpdateGraveyard(func(st *State) error {
		s.commitOutcome(st, o)
		return nil
	})
}

func (s *Service) commitOutcome(st *State, o Outcome) {
	if o.Action == ActionSkipped || o.Action == ActionProposed {
		return
	}
	idx := st.Index(o.Item.ID)
	if idx < 0 {
		return
	}
	current := &st.Items[idx]
	if current.Rev != o.Item.Rev {
		return
	}
	if o.Action != ActionRetry {
		dropWatchBurial(st, current.ID)
		st.Items = append(st.Items[:idx], st.Items[idx+1:]...)
		return
	}
	current.Path = o.Path
	current.Attempts++
	current.LastError = o.Detail
	current.NextAttempt = s.now().Add(backoff(current.Attempts))
	current.Rev++
}

func backoff(attempts int) time.Duration {
	d := time.Minute
	for i := 1; i < attempts && d < maxBackoff; i++ {
		d *= 2
	}
	return min(d, maxBackoff)
}
