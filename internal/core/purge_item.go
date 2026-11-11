package core

import (
	"errors"
	"fmt"
	"os"
)

type purgeStep uint8

const (
	purgeRefuse purgeStep = iota
	purgeMissing
	purgeEmpty
	purgeDelete
)

type purgeDecision struct {
	step   purgeStep
	dir    string
	detail string
}

type purgeObservation struct {
	dir                   string
	graveErr, verifyErr   error
	missing, recoveryGone bool
}

// Keep the existing observation order, including graveDir's legacy owner
// migration. Classification below is read-only; cleanup and deletion follow it.
func (s *Service) observePurge(b Burial, selected bool) purgeObservation {
	var observed purgeObservation
	observed.dir, observed.graveErr = s.graveDir(b)
	if observed.graveErr != nil {
		return observed
	}
	_, err := os.Lstat(observed.dir)
	observed.missing = s.missingOnOnlineVolume(observed.dir, err, burialVolume(b))
	if observed.missing {
		return observed
	}
	_, observed.verifyErr = s.verifyGrave(b)
	if !errors.Is(observed.verifyErr, errEmptyGrave) || !selected && b.PurgeIntent == nil {
		return observed
	}
	manifest := observed.dir + ".json"
	_, err = os.Lstat(manifest)
	observed.recoveryGone = s.missingOnOnlineVolume(manifest, err, burialVolume(b))
	return observed
}

// decidePurge only classifies observations. It cannot alter a grave or ledger.
func decidePurge(b Burial, selected bool, observed purgeObservation) purgeDecision {
	decision := purgeDecision{dir: observed.dir}
	if err := observed.graveErr; err != nil && !errors.Is(err, ErrNotFound) {
		decision.detail = "refusing to purge: " + err.Error()
		return decision
	}
	if errors.Is(observed.graveErr, ErrNotFound) || observed.missing {
		decision.step = purgeMissing
		return decision
	}
	if err := observed.verifyErr; err != nil {
		if !errors.Is(err, errEmptyGrave) || !selected && b.PurgeIntent == nil {
			decision.detail = "refusing to purge: " + err.Error()
			return decision
		}
		// An empty grave alone does not prove that a restore intent finished.
		if !observed.recoveryGone {
			decision.detail = "empty grave has pending recovery evidence; run obit reap first"
			return decision
		}
		decision.step = purgeEmpty
		return decision
	}
	decision.step = purgeDelete
	return decision
}

func (s *Service) requirePurgeAuditSpace(b Burial, dryRun bool) error {
	if dryRun || b.PurgeIntent != nil {
		return nil
	}
	current, err := s.Store.ReadSnapshot()
	if err != nil {
		return err
	}
	return current.requireAuditSpace()
}

func (s *Service) purgeBurial(r *GraveResult, state State, selected, all, dryRun bool) error {
	if r.Burial.Kind == KindContainer {
		return s.purgeContainer(r, dryRun)
	}
	if err := s.requirePurgeAuditSpace(*r.Burial, dryRun); err != nil {
		return err
	}
	observed := s.observePurge(*r.Burial, selected)
	decision := decidePurge(*r.Burial, selected, observed)
	return s.executePurge(r, state, decision, purgeBasis(all, selected), dryRun)
}

func (s *Service) executePurge(r *GraveResult, state State, decision purgeDecision, basis string, dryRun bool) error {
	b := r.Burial
	switch decision.step {
	case purgeRefuse:
		r.Error = decision.detail
		return nil
	case purgeMissing:
		r.Action = ActionLost
	case purgeEmpty:
		if !dryRun {
			err := s.discardEmptyGrave(decision.dir+".json", *b)
			if err != nil {
				r.Error = err.Error()
				return nil
			}
		}
		r.Action = ActionLost
	case purgeDelete:
		if dryRun {
			r.Action = ActionPurged
			return nil
		}
		err := s.deletePurgeBurial(r, state, decision.dir, basis)
		if err != nil || r.Error != "" {
			return err
		}
	}
	if r.Action == ActionLost && b.PurgeIntent != nil {
		r.Action = ActionPurged
	}
	if dryRun {
		return nil // a lost grave is only reported, never reconciled
	}
	if r.Action == ActionLost {
		err := s.cleanRecordedManifest(*b, state)
		if err != nil {
			r.Action, r.Error = "", err.Error()
			return nil
		}
	}
	return s.commitPurge(*r)
}

// Publish the intent before removal. Each failure keeps its original receipt
// semantics: manifest/delete failures retry this item; intent failures abort.
func (s *Service) deletePurgeBurial(r *GraveResult, state State, dir, basis string) error {
	err := s.cleanRecordedManifest(*r.Burial, state)
	if err != nil {
		r.Error = err.Error()
		return nil
	}
	err = s.beginPurge(r.Burial, basis)
	if err != nil {
		return err
	}
	err = os.RemoveAll(dir)
	if err != nil {
		r.Error = fmt.Sprintf("#%d: %v", r.Burial.ID, err)
		return nil
	}
	r.Action = ActionPurged
	return nil
}

func (s *Service) commitPurge(r GraveResult) error {
	b := r.Burial
	return s.Store.UpdateGraveyard(func(st *State) error {
		if j := st.BurialIndex(b.ID); j >= 0 {
			st.Burials = append(st.Burials[:j], st.Burials[j+1:]...)
		}
		if r.Action == ActionPurged {
			st.Stats.Purged++
			st.Stats.PurgedBytes += b.Size
		}
		st.queueAudit(Event{Time: s.now(), Action: r.Action, ID: b.ID, Path: b.Path, Detail: b.Grave})
		return nil
	})
}
