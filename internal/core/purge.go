package core

import (
	"context"
	"errors"
)

func (s *Service) beginPurge(b *Burial, basis string) error {
	intent := b.PurgeIntent
	if intent == nil {
		intent = &PurgeIntent{VerifiedAt: s.now(), Basis: basis}
	}
	// A visible intent may come from an unconfirmed replacement; confirm it
	// under the ledger lock before authorizing another permanent deletion.
	if err := s.Store.Confirm(func(st *State) error {
		idx := st.BurialIndex(b.ID)
		if idx < 0 || !sameBurial(st.Burials[idx], *b) {
			return ErrUnsafe
		}
		if b.PurgeIntent == nil {
			if err := st.requireAuditSpace(); err != nil {
				return err
			}
		}
		st.Burials[idx].PurgeIntent = intent
		return nil
	}); err != nil {
		return err
	}
	b.PurgeIntent = intent
	if s.beforePurgeDelete != nil {
		s.beforePurgeDelete()
	}
	return nil
}

// Purge deletes, for good, burials whose retention has passed (all of them
// with all). This is the only permanent deletion in Obituary, and it only
// ever removes a grave directory that verifyGrave vouches for.
func (s *Service) Purge(all, dryRun bool) ([]GraveResult, error) {
	return s.purge(all, dryRun, false)
}

// PurgeRefs acts only on explicitly selected records, including an unresolved
// resurrection that the user has chosen to abandon.
func (s *Service) PurgeRefs(refs []string, dryRun bool) ([]GraveResult, error) {
	return s.purgeSelected(false, dryRun, false, refs)
}

func (s *Service) purge(all, dryRun, background bool) ([]GraveResult, error) {
	return s.purgeSelected(all, dryRun, background, nil)
}

func (s *Service) purgeSelected(all, dryRun, background bool, refs []string) (results []GraveResult, finalErr error) {
	// Purge must finish its ledger commit after permanent deletion; neither
	// the explicit nor background entry point accepts request cancellation.
	s = s.WithContext(context.WithoutCancel(s.Store.Context()))
	s, flushAudit := s.auditBatch(dryRun)
	defer func() { finalErr = errors.Join(finalErr, flushAudit()) }()
	run := purgeRun{all: all, dryRun: dryRun, background: background, refs: refs}
	err := s.withGraveyardLock(func() error { return s.purgeLocked(&run) })
	return run.receipts(err), errors.Join(err, run.auditErr)
}

func purgeBasis(all, selected bool) string {
	if selected {
		return "explicit selection"
	}
	if all {
		return "explicit all"
	}
	return "retention elapsed"
}

func (s *Service) retryPurge(b *Burial, detail string) error {
	return s.Store.UpdateGraveyard(func(st *State) error {
		if idx := st.BurialIndex(b.ID); idx >= 0 {
			current := &st.Burials[idx]
			if current.NextPurgeAttempt.After(s.now()) && current.LastPurgeError == detail {
				*b = *current
				return nil
			}
			current.PurgeAttempts++
			current.NextPurgeAttempt = s.now().Add(backoff(current.PurgeAttempts))
			current.LastPurgeError = detail
			*b = *current
		}
		return nil
	})
}
