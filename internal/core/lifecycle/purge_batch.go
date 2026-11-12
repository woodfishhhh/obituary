package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Tick already owns graveyard.lock, excluding other burial/restore/purge
// writers. Persist all verified intents before any deletion, then retire a
// bounded prefix in one commit. Failed commits leave the existing purgeIntent
// protocol sufficient to distinguish deleted content from a lost grave.
func (s *Service) purgeExpiredBatches(state State, requested []Burial, blocked map[int64]string) (results []GraveResult, finalErr error) {
	for len(requested) > 0 {
		n := min(len(requested), sweepBatchSize)
		batch := make([]GraveResult, n)
		for i, b := range requested[:n] {
			batch[i] = s.inspectExpiredPurge(b, state, blocked[b.ID])
		}
		reserved, capacityErr, err := s.reservePurgeBatch(batch)
		if err != nil {
			for _, r := range batch {
				r.Action, r.Error = ActionNotExecuted, err.Error()
				results = append(results, r)
			}
			return results, err // an unconfirmed intent never authorizes deletion
		}
		batch = batch[:reserved]
		processed := s.deletePurgePrefix(batch, state)
		// Carry durable intents for the unprocessed suffix into its next pass.
		for i := processed; i < reserved; i++ {
			requested[i] = *batch[i].Burial
		}
		batch = batch[:processed]
		auditCount := 0
		if len(batch) > 0 {
			auditCount, err = s.commitPurgeBatch(batch)
		}
		if commitFailed(err) {
			for i := range batch {
				batch[i].Error = err.Error()
			}
			return append(results, batch...), err
		}
		results = append(results, batch...)
		finalErr = errors.Join(finalErr, err)
		requested = requested[processed:]
		if capacityErr != nil {
			return results, errors.Join(finalErr, capacityErr)
		}
		if auditCount >= maxPendingAudit-sweepBatchSize {
			if err := s.Store.FlushAudit(); err != nil {
				return results, errors.Join(finalErr, err)
			}
		}
	}
	return results, finalErr
}

func (s *Service) reservePurgeBatch(batch []GraveResult) (reserved int, capacityErr, err error) {
	err = s.Store.Confirm(func(st *State) error {
		lostReservations := 0
		for i := range batch {
			r, b := &batch[i], batch[i].Burial
			idx := st.BurialIndex(b.ID)
			if idx < 0 || !sameBurial(st.Burials[idx], *b) {
				return ErrUnsafe
			}
			if r.Error == "" && b.PurgeIntent == nil {
				if pendingAuditCount(st)+lostReservations >= maxPendingAudit {
					capacityErr = errAuditBacklog
					break
				}
				if r.Action == ActionLost {
					lostReservations++
				} else {
					intent := &PurgeIntent{VerifiedAt: s.now(), Basis: "retention elapsed"}
					st.Burials[idx].PurgeIntent, b.PurgeIntent = intent, intent
				}
			}
			reserved++
		}
		return nil
	})
	return reserved, capacityErr, err
}

func (s *Service) deletePurgePrefix(batch []GraveResult, state State) (processed int) {
	started := time.Now()
	for i := range batch {
		if processed > 0 && time.Since(started) >= sweepBatchWork {
			break
		}
		r := &batch[i]
		b := r.Burial
		processed++
		if r.Error != "" || r.Action == ActionLost {
			continue
		}
		// Verify again immediately before deletion; an earlier item's large
		// tree must not turn the next item's verification into a stale check.
		current := s.inspectExpiredPurge(*b, state, "")
		if current.Error != "" {
			*r = current
			continue
		}
		if current.Action == ActionLost || current.Action == ActionPurged {
			r.Action = ActionPurged
			continue
		}
		if s.beforePurgeDelete != nil {
			s.beforePurgeDelete()
		}
		if err := os.RemoveAll(filepath.Dir(b.Grave)); err != nil {
			r.Error = fmt.Sprintf("#%d: %v", b.ID, err)
		} else {
			r.Action = ActionPurged
		}
	}
	return processed
}

func (s *Service) commitPurgeBatch(batch []GraveResult) (auditCount int, err error) {
	err = s.Store.UpdateGraveyard(func(st *State) error {
		for i := range batch {
			r, b := &batch[i], batch[i].Burial
			idx := st.BurialIndex(b.ID)
			if idx < 0 || !sameBurial(st.Burials[idx], *b) {
				return ErrUnsafe
			}
			if r.Error != "" {
				current := &st.Burials[idx]
				if !current.NextPurgeAttempt.After(s.now()) || current.LastPurgeError != r.Error {
					current.PurgeAttempts++
					current.NextPurgeAttempt = s.now().Add(backoff(current.PurgeAttempts))
					current.LastPurgeError = r.Error
				}
				*b = *current
				r.Action = ActionBlocked
				continue
			}
			if b.PurgeIntent != nil {
				r.Action = ActionPurged
			}
			st.Burials = append(st.Burials[:idx], st.Burials[idx+1:]...)
			if r.Action == ActionPurged {
				st.Stats.Purged++
				st.Stats.PurgedBytes += b.Size
			}
			queueAudit(st, Event{Time: s.now(), Action: r.Action, ID: b.ID, Path: b.Path, Detail: b.Grave})
		}
		auditCount = len(st.PendingAudit)
		return nil
	})
	return auditCount, err
}

// A blank action means a verified, nonempty grave ready for a durable intent.
func (s *Service) inspectExpiredPurge(b Burial, state State, blocked string) GraveResult {
	r := GraveResult{Ref: fmt.Sprintf("#%d", b.ID), Burial: &b}
	if blocked != "" {
		r.Action, r.Error = ActionBlocked, blocked
		return r
	}
	dir, err := s.graveDir(b)
	if err != nil && !errors.Is(err, ErrNotFound) {
		r.Error = "refusing to purge: " + err.Error()
		return r
	}
	if errors.Is(err, ErrNotFound) {
		r.Action = ActionLost
	} else if _, err := os.Lstat(dir); s.missingOnOnlineVolume(dir, err, burialVolume(b)) {
		r.Action = ActionLost
	} else if _, err := s.verifyGrave(b); err != nil {
		if !s.inspectEmptyPurge(&r, b, dir, err) {
			return r
		}
	}
	if r.Action == ActionLost && b.PurgeIntent != nil {
		r.Action = ActionPurged
	}
	if err := s.cleanRecordedManifest(b, state); err != nil {
		r.Action, r.Error = "", err.Error()
	}
	return r
}

func (s *Service) inspectEmptyPurge(r *GraveResult, b Burial, dir string, err error) bool {
	if !errors.Is(err, errEmptyGrave) || b.PurgeIntent == nil {
		r.Error = "refusing to purge: " + err.Error()
		return false
	}
	manifest := dir + ".json"
	if _, err := os.Lstat(manifest); !s.missingOnOnlineVolume(manifest, err, burialVolume(b)) {
		r.Error = "empty grave has pending recovery evidence; run obit reap first"
		return false
	}
	if err := s.discardEmptyGrave(manifest, b); err != nil {
		r.Error = err.Error()
		return false
	}
	r.Action = ActionLost
	return true
}
