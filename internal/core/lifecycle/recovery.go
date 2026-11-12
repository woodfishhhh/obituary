package lifecycle

import (
	"errors"
	"fmt"
	"os"
)

// Children records identities before the move; Contains is the set still in
// the grave. Departed children stay untracked. Uncertain locations block recovery.
func (s *Service) reconcileChildren(st *State, b *Burial) error {
	var contained []int64
	for _, child := range b.Children {
		idx := st.Index(child.ID)
		if st.BurialIndex(child.ID) >= 0 || st.ProposalIndex(child.ID) >= 0 ||
			idx >= 0 && (st.Items[idx].Identity == nil || *st.Items[idx].Identity != *child.Identity) {
			return fmt.Errorf("contained #%d belongs to another object: %w", child.ID, ErrUnsafe)
		}
		current, err := s.FS.Locate(*child.Identity)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("locate contained #%d: %w", child.ID, err)
		}
		grave, err := s.FS.RealPath(b.Grave)
		if err != nil {
			return err
		}
		if isInside(current, grave) {
			contained = append(contained, child.ID)
		}
	}
	b.Contains = contained
	return nil
}

type recovery struct {
	burials   []Burial
	blocked   map[int64]bool
	roots     map[string]string
	details   map[int64]string
	outcomes  []Outcome
	locations []resolvedGraveyardRoot
}

// Even rejected content can name parent/child records that need protection.
func (s *Service) manifestIDs(st State, p PendingGrave) []int64 {
	ids := []int64{p.id}
	b := p.Burial
	if b == nil {
		b = p.partial
	}
	if b != nil {
		if b.ID > 0 && b.ID != p.id {
			ids = append(ids, b.ID)
		}
		ids = append(ids, b.Contains...)
		for _, child := range b.Children {
			ids = append(ids, child.ID)
			if child.Identity != nil {
				if idx := st.FindIdentity(*child.Identity); idx >= 0 {
					ids = append(ids, st.Items[idx].ID)
				}
			}
		}
	}
	if p.root != "" {
		for _, it := range st.Items {
			if samePath(s.graveyardRoot(it.Path), p.root) || it.Identity != nil && samePath(s.graveyardRoot(it.Identity.Volume), p.root) {
				ids = append(ids, it.ID)
			}
		}
		for _, b := range st.Burials {
			if samePath(s.graveyardRoot(b.Path), p.root) {
				ids = append(ids, b.ID)
			}
		}
	}
	return ids
}

// Caller owns sweep.lock. Serialize recovery with resurrection/purge, too.
// Ledger read/commit failures abort the batch. Inspection and metadata cleanup
// failures block only their receipt; audit warnings accompany confirmed results.
func (s *Service) recoverManifests(paths ...string) (recovery, error) {
	r := recovery{blocked: make(map[int64]bool), roots: make(map[string]string), details: make(map[int64]string)}
	var warnings error
	err := s.withGraveyardLock(func() error {
		var err error
		warnings, err = s.recoverManifestBatch(&r, paths)
		return err
	})
	if err != nil {
		return r, err
	}
	if err := s.sizeRecoveredBurials(r.burials); commitFailed(err) {
		return r, errors.Join(warnings, fmt.Errorf("burial confirmed; updating recovered burial sizes failed: %w", err))
	} else {
		warnings = errors.Join(warnings, err)
	}
	r.updateOutcomeSizes()
	return r, warnings
}

func (s *Service) recoverManifestBatch(r *recovery, paths []string) (warnings, fatal error) {
	st, err := s.Store.Read()
	if commitFailed(err) {
		return nil, err
	}
	warnings = errors.Join(warnings, err)
	r.locations = s.resolveGraveyardRoots(st, paths...)
	pending, _ := s.scanGraveyardRoots(st, r.locations) // failures have per-root receipts
	for _, p := range pending {
		result, err := s.recoverManifest(p)
		if commitFailed(err) {
			return warnings, err
		}
		warnings = errors.Join(warnings, err)
		if result.burial != nil {
			r.burials = append(r.burials, *result.burial)
		}
		if result.outcome != nil {
			r.outcomes = append(r.outcomes, *result.outcome)
		}
		if result.warning != nil {
			warnings = errors.Join(warnings, s.warnRecovery(r, st, p, result.warning))
		}
	}
	retryWarnings, err := s.retryBlockedRecovery(r, st)
	return errors.Join(warnings, retryWarnings), err
}

// A single receipt's warning is local. The separate error is a ledger failure
// (abort) or an audit warning (keep the confirmed result and continue).
type manifestRecovery struct {
	burial  *Burial
	outcome *Outcome
	warning error
}

func (s *Service) recoverManifest(p PendingGrave) (manifestRecovery, error) {
	switch p.Status {
	case "resurrected", "restore-ready", "restore-committed", "restore-abandoned":
		return s.recoverRestoreManifest(p)
	case "recorded":
		return s.recoverRecordedManifest(p)
	case "empty":
		return manifestRecovery{warning: s.discardEmptyGrave(p.Manifest, *p.Burial)}, nil
	case "pending":
		return s.recoverPendingManifest(p)
	default:
		return manifestRecovery{warning: errors.New(p.Error)}, nil
	}
}

func (s *Service) recoverRestoreManifest(p PendingGrave) (manifestRecovery, error) {
	restored, err := s.recoverRestoreIntent(p)
	if commitFailed(err) {
		return manifestRecovery{}, err
	}
	result := manifestRecovery{warning: restored.Warning}
	if restored.Committed {
		result.outcome = &Outcome{Item: Item{ID: p.Burial.ID}, Action: ActionResurrected,
			Path: restored.Path, Burial: p.Burial, Detail: errorText(restored.Warning)}
	}
	return result, err
}

func (s *Service) recoverRecordedManifest(p PendingGrave) (manifestRecovery, error) {
	confirmed, checkErr, err := s.confirmRecordedBurial(*p.Burial)
	if checkErr != nil {
		return manifestRecovery{warning: checkErr}, nil
	}
	if commitFailed(err) {
		return manifestRecovery{}, err
	}
	result := recoveredManifestBurial(confirmed, "confirmed from "+p.Manifest)
	// At least once: failed audit/metadata cleanup retains the evidence,
	// so a later confirmation may repeat the same numbered event.
	if err != nil {
		return result, err
	}
	result.warning = os.Remove(p.Manifest)
	return result, nil
}

func (s *Service) recoverPendingManifest(p PendingGrave) (manifestRecovery, error) {
	b := *p.Burial
	var checkErr error
	err := s.Store.UpdateGraveyard(func(st *State) error { checkErr = s.entomb(st, &b); return checkErr })
	if checkErr != nil {
		return manifestRecovery{warning: checkErr}, nil
	}
	if commitFailed(err) {
		return manifestRecovery{}, err
	}
	result := recoveredManifestBurial(b, "recovered from "+p.Manifest)
	result.warning = os.Remove(p.Manifest)
	return result, err
}

func recoveredManifestBurial(b Burial, detail string) manifestRecovery {
	return manifestRecovery{burial: &b, outcome: &Outcome{
		Item:   Item{ID: b.ID, Path: b.Path, Identity: b.Identity},
		Action: ActionBuried, Path: b.Path, Grave: b.Grave, GraveID: b.GraveID,
		Burial: &b, Detail: detail,
	}}
}

func (r *recovery) block(id int64, path string, err error) {
	r.blocked[id] = true
	r.details[id] = err.Error()
	r.outcomes = append(r.outcomes, Outcome{Item: Item{ID: id}, Action: ActionBlocked, Path: path, Detail: err.Error()})
}

func (s *Service) blockRecoveryRoot(r *recovery, st State, root string, err error) {
	r.roots[ProjectKey(root)] = err.Error()
	before := len(r.outcomes)
	for _, it := range st.Items {
		if samePath(s.graveyardRoot(it.Path), root) || it.Identity != nil && samePath(s.graveyardRoot(it.Identity.Volume), root) {
			r.block(it.ID, it.Path, err)
		}
	}
	for _, b := range st.Burials {
		if samePath(s.graveyardRoot(b.Path), root) {
			r.block(b.ID, b.Grave, err)
		}
	}
	if len(r.outcomes) == before {
		r.block(0, root, err)
	}
}

func (s *Service) warnRecovery(r *recovery, st State, p PendingGrave, err error) error {
	if p.root != "" {
		s.blockRecoveryRoot(r, st, p.root, err)
	} else {
		r.block(p.id, p.Manifest, err)
	}
	ids := s.manifestIDs(st, p)
	for _, id := range ids {
		r.blocked[id] = true
		r.details[id] = err.Error()
	}
	for _, id := range ids {
		idx := st.BurialIndex(id)
		if idx < 0 {
			continue
		}
		b := st.Burials[idx]
		if b.NextPurgeAttempt.After(s.now()) && b.LastPurgeError == err.Error() {
			return nil
		}
	}
	return s.Store.Record(Event{Time: s.now(), Action: "recovery warning", ID: p.id, Path: p.Manifest, Detail: err.Error()})
}

// Retry commits preserve audit warnings; any purge-backoff error aborts just as
// a failed item commit does. Neither failure is converted into a local warning.
func (s *Service) retryBlockedRecovery(r *recovery, st State) (warnings, fatal error) {
	for _, it := range st.Items {
		if !r.blocked[it.ID] || !it.Due(s.now()) {
			continue
		}
		err := s.commit(Outcome{Item: it, Path: it.Path, Action: ActionRetry, Detail: r.details[it.ID]})
		if commitFailed(err) {
			return warnings, err
		}
		warnings = errors.Join(warnings, err)
	}
	for _, b := range st.Burials {
		if !r.blocked[b.ID] || b.PurgeAt.After(s.now()) || b.NextPurgeAttempt.After(s.now()) {
			continue
		}
		if err := s.retryPurge(&b, r.details[b.ID]); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func (r *recovery) updateOutcomeSizes() {
	for i := range r.outcomes {
		if r.outcomes[i].Action != ActionBuried {
			continue
		}
		for _, b := range r.burials {
			if b.ID == r.outcomes[i].Item.ID {
				r.outcomes[i].Size, r.outcomes[i].Burial = b.Size, &b
				break
			}
		}
	}
}
