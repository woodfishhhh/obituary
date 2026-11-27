package lifecycle

import (
	"errors"
	"fmt"
	"time"
)

// ApproveQuotaProposal buries a quota-pending object once it fits within the
// current graveyard capacity limit; otherwise it refuses without deleting anything.
func (s *Service) ApproveQuotaProposal(p Proposal) (Outcome, error) {
	p, err := s.verifyQuotaProposal(p.ID, ProposalFingerprint(p))
	if err != nil {
		return Outcome{}, err
	}
	path := p.Candidates[0].Path
	size, err := measureLocalSize(path)
	if err != nil {
		return Outcome{Path: path}, err
	}
	if s.GraveyardMaxBytes > 0 && size > s.GraveyardMaxBytes {
		return Outcome{Path: path, Action: ActionQuotaExceeded, Size: size}, fmt.Errorf("%s (%s) still exceeds graveyard capacity limit %s; raise graveyard.max_size, extend, reject, or explicitly confirm permanent deletion",
			path, formatQuotaBytes(size), formatQuotaBytes(s.GraveyardMaxBytes))
	}
	it, tracked, err := s.verifyQuotaTrackedItem(p)
	if err != nil {
		return Outcome{Path: path}, err
	}
	if tracked && (it.Source == "watch" || automatic(it)) {
		return s.approveAutomaticQuotaItem(p, it, path)
	}
	return s.approveDirectQuotaBurial(p, path, tracked && it.Force)
}

func (s *Service) verifyQuotaTrackedItem(p Proposal) (Item, bool, error) {
	if p.QuotaReview == nil || p.QuotaReview.ItemID == 0 {
		return Item{}, false, nil
	}
	st, err := s.Store.ReadHot()
	if err != nil {
		return Item{}, false, err
	}
	idx := st.Index(p.QuotaReview.ItemID)
	if idx < 0 {
		return Item{}, false, ErrProposalChanged
	}
	it := st.Items[idx]
	if it.Rev != p.QuotaReview.ItemRev || it.Identity == nil || *it.Identity != p.QuotaReview.Identity {
		return Item{}, false, ErrProposalChanged
	}
	return it, true, nil
}

func (s *Service) approveAutomaticQuotaItem(p Proposal, it Item, path string) (Outcome, error) {
	s, notify := s.trackChanges()
	defer notify()
	st, err := s.Store.Read()
	if err != nil {
		return Outcome{Path: path}, err
	}
	tx := *s
	id := p.QuotaReview.Identity
	tx.expectedIdentity = &id
	reason := ReasonExpired
	if it.Source == "watch" && p.WatchBurial != nil {
		tx.approvedWatch, tx.Mode, reason = &p, ModeGraveyard, ReasonBury
	} else if it.Source != "watch" && !it.Due(s.now()) {
		if verdict, detail := tx.recheckAutomatic(it, path); verdict != "" {
			return Outcome{Item: it, Path: path, Action: verdict, Detail: detail}, errors.New(detail)
		}
		reason = ReasonBury
	}
	roots := tx.resolveGraveyardRoots(st, it.Path)
	out := tx.settle(it, false, reason, roots)
	return s.finishAutomaticQuotaApproval(p, it, out, &tx)
}

func (s *Service) finishAutomaticQuotaApproval(p Proposal, it Item, out Outcome, tx *Service) (Outcome, error) {
	if out.Action == ActionSkipped && out.Detail == ErrProposalChanged.Error() {
		return out, ErrProposalChanged
	}
	if out.Action == ActionProposed {
		dropErr := s.Store.UpdateHot(func(st *State) error {
			dropQuotaProposal(st, p.QuotaReview.ItemID, &p.QuotaReview.Identity)
			return nil
		})
		return out, errors.Join(out.err, dropErr)
	}
	if out.Action == ActionBuried && s.GraveyardMaxBytes > 0 {
		ev, evErr := s.EnforceGraveyardQuota(false)
		if ev.PurgedCount > 0 || ev.FailedCount > 0 {
			out.QuotaEviction = ev.summary()
		}
		out.err = errors.Join(out.err, evErr)
		return out, out.err
	}
	if out.Action == ActionRecycled {
		if err := tx.commit(out); err != nil {
			return out, err
		}
		return out, out.err
	}
	if out.Action != ActionRetry {
		return out, out.err
	}
	if !posixWatchFile(it) {
		if err := tx.commit(out); err != nil {
			return out, err
		}
	}
	return out, errors.Join(out.err, s.Store.RecordCommitted(Event{Time: s.now(), Action: out.Action, ID: it.ID, Path: out.Path, Detail: out.Detail}))
}

func (s *Service) approveDirectQuotaBurial(p Proposal, path string, force bool) (Outcome, error) {
	tx := *s
	id := p.QuotaReview.Identity
	tx.expectedIdentity = &id
	results, err := tx.Bury([]string{path}, force, "approved quota decision")
	if err != nil && !OnlyAuditErrors(err) {
		return Outcome{Path: path}, err
	}
	if len(results) == 0 {
		return Outcome{Path: path}, errors.New("no burial result returned")
	}
	r := results[0]
	if errors.Is(r.Err, ErrProposalChanged) {
		return Outcome{Path: path, Detail: r.Error}, ErrProposalChanged
	}
	out := Outcome{Path: path, Action: r.Action, Burial: r.Burial, Detail: r.Error, QuotaOverflow: r.QuotaOverflow, QuotaEviction: r.QuotaEviction}
	if r.Burial != nil {
		out.Item = Item{ID: r.Burial.ID, Path: r.Burial.Path, IsDir: r.Burial.IsDir}
	}
	return out, err
}

// PurgeQuotaProposal permanently deletes ONLY the reviewed oversized object by
// pinning it into an exclusive grave and running verified Purge on that grave
// alone, never calling os.RemoveAll on user content and never evicting old graves.
func (s *Service) PurgeQuotaProposal(ref, fingerprint string) (ProposalResult, error) {
	r := ProposalResult{Ref: ref}
	id, err := parseProposalRef(ref)
	if err != nil {
		r.Error = err.Error()
		return r, err
	}
	release, err := s.lockProposals()
	if err != nil {
		r.Error = err.Error()
		return r, err
	}
	defer release()
	p, err := s.verifyQuotaProposal(id, fingerprint)
	if err != nil {
		return quotaProposalErr(r, err), err
	}
	r.Proposal = p
	it, tracked, err := s.verifyQuotaTrackedItem(p)
	if err != nil {
		return quotaProposalErr(r, err), err
	}
	path := p.Candidates[0].Path
	if err := s.verifyQuotaLiveObject(path, p.QuotaReview.Identity); err != nil {
		return quotaProposalErr(r, err), err
	}
	if tracked && automatic(it) {
		if verdict, detail := s.recheckAutomatic(it, path); verdict != "" {
			err = errors.New(detail)
			return quotaProposalErr(r, err), err
		}
	}
	buried, err := s.buryQuotaOnly(path, tracked && it.Force, p.QuotaReview.Identity)
	if err != nil {
		return quotaProposalErr(r, err), err
	}
	if err := s.purgeQuotaSingleGrave(buried.ID); err != nil {
		return quotaProposalErr(r, err), err
	}
	r.Results = []Result{{Ref: path, Action: ActionPurged}}
	return r, nil
}

// ExtendQuotaProposal gives a quota-pending object a fresh future lifespan and
// clears its pending decision without moving or deleting the object.
func (s *Service) ExtendQuotaProposal(ref, fingerprint string, d time.Duration) (ProposalResult, error) {
	r := ProposalResult{Ref: ref}
	id, err := parseProposalRef(ref)
	if err != nil {
		r.Error = err.Error()
		return r, err
	}
	if d <= 0 || d > MaxLifespan {
		err = fmt.Errorf("lifespan must be positive and at most %s", MaxLifespan)
		r.Error = err.Error()
		return r, err
	}
	release, err := s.lockProposals()
	if err != nil {
		r.Error = err.Error()
		return r, err
	}
	defer release()
	p, err := s.verifyQuotaProposal(id, fingerprint)
	if err != nil {
		return quotaProposalErr(r, err), err
	}
	r.Proposal = p
	results, err := s.extendQuotaTarget(p, d)
	r.Results = results
	if err != nil {
		r = quotaProposalErr(r, err)
	} else if Failed(results) {
		r.Error = results[0].Error
	}
	return r, err
}

func (s *Service) extendQuotaTarget(p Proposal, d time.Duration) ([]Result, error) {
	if p.QuotaReview.ItemID > 0 {
		if _, _, err := s.verifyQuotaTrackedItem(p); err != nil {
			return nil, err
		}
		return s.Extend([]string{fmt.Sprint(p.QuotaReview.ItemID)}, d)
	}
	path := p.Candidates[0].Path
	want := p.QuotaReview.Identity
	if err := s.verifyQuotaLiveObject(path, want); err != nil {
		return nil, err
	}
	results, err := s.Add([]string{path}, AddOptions{
		ExpiresAt: s.now().Add(d),
		Source:    "cli",
		Project:   p.Project,
		Note:      fmt.Sprintf("extended from quota decision #%d", p.ID),
		BeforeCommit: func() error {
			return s.verifyQuotaLiveObject(path, want)
		},
	})
	if err != nil && !OnlyAuditErrors(err) {
		return results, err
	}
	if !Failed(results) {
		dropErr := s.Store.UpdateHot(func(st *State) error {
			dropQuotaProposal(st, p.QuotaReview.ItemID, &p.QuotaReview.Identity)
			return nil
		})
		err = errors.Join(err, dropErr)
	}
	return results, err
}

func (s *Service) verifyQuotaProposal(id int64, fingerprint string) (Proposal, error) {
	st, err := s.Store.ReadHot()
	if err != nil {
		return Proposal{}, err
	}
	idx := st.ProposalIndex(id)
	if idx < 0 {
		return Proposal{}, fmt.Errorf("#%d is not a pending proposal", id)
	}
	p := st.Proposals[idx]
	if p.Kind != ProposalGraveyardQuota || p.QuotaReview == nil || len(p.Candidates) != 1 {
		return Proposal{}, fmt.Errorf("#%d is not a graveyard quota decision", id)
	}
	if err := checkProposalReview(p, fingerprint); err != nil {
		return Proposal{}, err
	}
	return p, nil
}

func (s *Service) verifyQuotaLiveObject(path string, want Identity) error {
	got, err := s.FS.Identify(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%s changed since review: %w", path, ErrProposalChanged)
	}
	return nil
}

func (s *Service) buryQuotaOnly(path string, force bool, identity Identity) (*Burial, error) {
	tx := *s
	tx.GraveyardMaxBytes = 0
	tx.Mode = ModeGraveyard
	tx.expectedIdentity = &identity
	results, err := tx.bury([]string{path}, force, "quota_exceeded_permanent_delete", "quota-decision")
	if err != nil && !OnlyAuditErrors(err) {
		return nil, err
	}
	if len(results) > 0 && errors.Is(results[0].Err, ErrProposalChanged) {
		return nil, ErrProposalChanged
	}
	if len(results) == 0 || results[0].Action != ActionBuried || results[0].Burial == nil {
		msg := "could not move oversized object into temporary grave"
		if len(results) > 0 && results[0].Error != "" {
			msg = results[0].Error
		}
		return nil, errors.New(msg)
	}
	return results[0].Burial, nil
}

func (s *Service) purgeQuotaSingleGrave(burialID int64) error {
	tx := *s
	tx.GraveyardMaxBytes = 0
	results, err := tx.PurgeRefs([]string{fmt.Sprint(burialID)}, false)
	if err != nil && !OnlyAuditErrors(err) {
		return err
	}
	if len(results) == 0 || results[0].Error != "" || results[0].Action != ActionPurged {
		msg := fmt.Sprintf("could not purge grave #%d", burialID)
		if len(results) > 0 && results[0].Error != "" {
			msg = results[0].Error
		}
		return errors.New(msg)
	}
	return nil
}

func quotaProposalErr(r ProposalResult, err error) ProposalResult {
	r.Error = err.Error()
	if errors.Is(err, ErrProposalChanged) {
		r.ErrorCode = ErrorCodeProposalChanged
	}
	return r
}

func (s *Service) rejectQuotaProposal(st *State, p Proposal, at time.Time) {
	if p.QuotaReview == nil {
		return
	}
	path := ""
	if len(p.Candidates) > 0 {
		path = p.Candidates[0].Path
	}
	auditID := p.ID
	if p.QuotaReview.ItemID > 0 {
		dropWatchBurial(st, p.QuotaReview.ItemID)
		if i := st.Index(p.QuotaReview.ItemID); i >= 0 && (st.Items[i].Identity == nil || *st.Items[i].Identity == p.QuotaReview.Identity) {
			it := st.Items[i]
			auditID = it.ID
			s.spareWatchObject(st, it.WatchID, it.Identity, it.Path, "spared")
			s.spareAutomaticObject(st, it.Identity, it.Path)
			s.forgetAutoReceipt(st, it)
			st.Items = append(st.Items[:i], st.Items[i+1:]...)
		}
	}
	queueAudit(st, Event{Time: at, Action: ActionCanceled, ID: auditID, Path: path, Detail: "graveyard_quota_decision_rejected: kept in place and stopped tracking"})
}
