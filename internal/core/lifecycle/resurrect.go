package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Resurrect moves verified burials back through pinned handles, using a
// "(resurrected)" name on collision; nothing is overwritten.
// Lock/read errors, audit-capacity failures, pre-write cancellation and returned
// recovery/commit errors abort the batch. Reference, grave, preparation (including
// recorded-manifest confirmation) and move failures only populate that ref's
// Error; cleanup warnings and committed audit errors do not stop later refs.
func (s *Service) Resurrect(refs []string) (results []GraveResult, finalErr error) {
	s, notify := s.trackChanges()
	defer notify()
	s, flushAudit := s.auditBatch(false)
	defer func() { finalErr = errors.Join(finalErr, flushAudit()) }()
	results = make([]GraveResult, len(refs))
	for i, ref := range refs {
		results[i] = GraveResult{Ref: ref, Action: ActionNotExecuted, Error: "not executed: an earlier operation failed"}
	}
	var auditErr error
	for i, ref := range refs {
		var err error
		results[i], err = s.resurrectRef(ref)
		if commitFailed(err) {
			results[i].Error = err.Error()
			return results, err
		}
		auditErr = errors.Join(auditErr, err)
	}
	return results, auditErr
}

// resurrectRef owns one graveyard lock. A returned error is batch-level (except
// committed audit warnings); a refusal carried in GraveResult.Error is per-ref.
func (s *Service) resurrectRef(ref string) (result GraveResult, err error) {
	result = GraveResult{Ref: ref}
	err = s.withGraveyardLock(func() error {
		var opErr error
		result, opErr = s.resurrectLocked(ref)
		return opErr
	})
	return result, err
}

func (s *Service) resurrectLocked(ref string) (GraveResult, error) {
	result := GraveResult{Ref: ref}
	state, err := s.Store.Read()
	if err != nil {
		return result, err
	}
	b, pending, lookupErr := s.findResurrection(state, ref)
	result.Burial = b
	if lookupErr != nil {
		result.Error = lookupErr.Error()
		return result, nil
	}
	if pending != nil {
		r, err := s.recoverRestoreIntent(*pending)
		if r.Committed {
			result.Action, result.Burial, result.Path = ActionResurrected, pending.Burial, r.Path
		}
		result.Warning = errorText(r.Warning)
		return result, err
	}
	idx := state.BurialIndex(b.ID)
	err = s.resurrectBurial(&result, state.Burials[idx], state)
	return result, err
}

// Existing intents are reconciled before a new move. Recovery transaction errors
// abort; verification, manifest preparation and filesystem refusals stay here.
func (s *Service) resurrectBurial(result *GraveResult, b Burial, state State) error {
	if b.Kind == KindContainer {
		return s.resurrectContainer(result, b)
	}
	if b.PurgeIntent != nil {
		result.Error = fmt.Sprintf("#%d: purge is in progress; retry obit purge %d", b.ID, b.ID)
		return nil
	}
	done, err := s.recoverRecordedResurrection(result, b, state)
	if done || err != nil {
		return err
	}
	if _, err := s.verifyGrave(b); err != nil {
		if errors.Is(err, ErrNotFound) {
			err = fmt.Errorf("#%d: its grave %s is empty (removed outside Obituary)", b.ID, b.Grave)
		}
		result.Error = err.Error()
		return nil
	}
	if err := requireAuditSpace(&state); err != nil {
		return err
	}
	if err := s.Store.Context().Err(); err != nil {
		return err
	}
	return s.restoreBurial(result, b, state)
}

// done also covers a blocked ready intent: its warning must not trigger a new
// move. Only a recovery transaction error escapes to the batch controller.
func (s *Service) recoverRecordedResurrection(result *GraveResult, b Burial, state State) (done bool, err error) {
	manifest := filepath.Dir(b.Grave) + ".json"
	if _, statErr := os.Lstat(manifest); statErr != nil {
		return false, nil
	}
	p := s.inspectManifest(s.graveyardRoot(b.Path), manifest, state)
	if p.restore == nil {
		return false, nil
	}
	if p.Status == "unverifiable" {
		result.Error = p.Error
		return true, nil
	}
	tx := s.WithContext(context.WithoutCancel(s.Store.Context()))
	if p.Status == "resurrected" {
		result.Action, result.Burial, result.Path = ActionResurrected, &b, p.Path
	}
	r, err := tx.recoverRestoreIntent(p)
	result.Warning = errorText(r.Warning)
	if err != nil {
		return true, err
	}
	if !r.Committed && r.Warning != nil {
		result.Action, result.Burial, result.Error = ActionBlocked, &b, r.Warning.Error()
	}
	return r.Committed || r.Warning != nil, nil
}
