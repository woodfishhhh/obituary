package lifecycle

import (
	"errors"
	"fmt"
	"time"
)

type extendTarget struct {
	ref string
	id  int64
	err error
}

// ExtendGraves pushes the cremation deadline of each referenced burial back
// by d (counted from now if retention has already elapsed). Active tracked
// items and burials currently being purged are refused in their result slots.
func (s *Service) ExtendGraves(refs []string, d time.Duration) ([]GraveResult, error) {
	if d <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	s, notify := s.trackChanges()
	defer notify()
	now := s.now()
	results := make([]GraveResult, len(refs))
	var events []Event
	var auditErr error
	err := s.withGraveyardLock(func() error {
		state, readErr := s.Store.Read()
		if commitFailed(readErr) {
			return readErr
		}
		auditErr = errors.Join(auditErr, readErr)
		targets, manifestErr := s.prepareExtendGraves(state, refs)
		return s.Store.UpdateGraveyard(func(st *State) error {
			events = s.applyExtendGraves(st, targets, now, d, manifestErr, results)
			return nil
		})
	})
	if commitFailed(err) {
		return nil, lifespanCommitError(err)
	}
	return results, errors.Join(auditErr, err, s.Store.RecordCommitted(events...))
}

func (s *Service) prepareExtendGraves(st State, refs []string) ([]extendTarget, map[int64]error) {
	targets := make([]extendTarget, len(refs))
	seen := make(map[int64]bool, len(refs))
	errs := make(map[int64]error)
	for i, ref := range refs {
		idx, err := s.findBurial(&st, ref)
		if err != nil {
			targets[i] = extendTarget{ref: ref, err: err}
			continue
		}
		if idx < 0 {
			targets[i] = extendTarget{ref: ref, err: missingBurial(ref)}
			continue
		}
		b := st.Burials[idx]
		targets[i] = extendTarget{ref: ref, id: b.ID}
		if seen[b.ID] || b.IsExternal() || b.PurgeIntent != nil || b.IdentityMismatch != "" {
			continue
		}
		seen[b.ID] = true
		if err := s.cleanRecordedManifest(b, st); err != nil {
			errs[b.ID] = err
		}
	}
	return targets, errs
}

func canExtendBurial(st *State, b *Burial, manifestErr map[int64]error) error {
	if b.PurgeIntent != nil {
		return fmt.Errorf("#%d: purge is in progress; cannot extend", b.ID)
	}
	if b.IdentityMismatch != "" {
		return fmt.Errorf("#%d: identity mismatch; cannot extend: %s", b.ID, b.IdentityMismatch)
	}
	if b.Kind == KindContainer && containerIntent(st, b.ID) != nil {
		return errors.New("container operation pending; run obit reap")
	}
	return manifestErr[b.ID]
}

func (s *Service) applyExtendGraves(st *State, targets []extendTarget, now time.Time, d time.Duration, manifestErr map[int64]error, results []GraveResult) []Event {
	var events []Event
	for i, target := range targets {
		results[i].Ref = target.ref
		if target.err != nil {
			results[i].Error, results[i].Err = target.err.Error(), target.err
			continue
		}
		idx := st.BurialIndex(target.id)
		if idx < 0 {
			results[i].Err = missingBurial(target.ref)
			results[i].Error = results[i].Err.Error()
			continue
		}
		b := &st.Burials[idx]
		if err := canExtendBurial(st, b, manifestErr); err != nil {
			results[i].Error, results[i].Err = err.Error(), err
			continue
		}
		extendBurialPurgeAt(b, now, d)
		after := *b
		results[i].Action, results[i].Burial, results[i].Path = ActionExtended, &after, after.Path
		events = append(events, Event{
			Time: now, Action: ActionExtended, ID: after.ID, Path: after.Path,
			Detail: "expires " + after.PurgeAt.Format(time.RFC3339),
		})
	}
	return events
}

func extendBurialPurgeAt(b *Burial, now time.Time, d time.Duration) {
	base := b.PurgeAt
	if base.Before(now) {
		base = now
	}
	b.PurgeAt = base.Add(d)
	if limit := now.Add(MaxLifespan); b.PurgeAt.After(limit) {
		b.PurgeAt = limit
	}
	b.PurgeAttempts, b.NextPurgeAttempt, b.LastPurgeError = 0, time.Time{}, ""
}
