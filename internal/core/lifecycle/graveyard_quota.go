package lifecycle

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

const (
	ProposalGraveyardQuota = ledger.ProposalGraveyardQuota
	ActionQuotaExceeded    = ledger.ActionQuotaExceeded
	PurgeBasisQuotaFIFO    = ledger.PurgeBasisQuotaFIFO
)

type QuotaBurialReview = ledger.QuotaBurialReview

// QuotaOverflow reports that one object exceeded the configured graveyard
// capacity limit and was left safely in place in pending decisions (Plan A).
type QuotaOverflow struct {
	ProposalID int64  `json:"proposalId,omitempty"`
	Path       string `json:"path"`
	ItemBytes  int64  `json:"itemBytes"`
	MaxBytes   int64  `json:"maxBytes"`
}

// QuotaEvictionItem describes one local graveyard entry selected for or purged
// by FIFO capacity eviction.
type QuotaEvictionItem struct {
	ID       int64     `json:"id"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	BuriedAt time.Time `json:"buriedAt"`
}

// QuotaEviction summarizes FIFO capacity eviction (previewed or executed).
type QuotaEviction struct {
	MaxBytes    int64               `json:"maxBytes"`
	BeforeBytes int64               `json:"beforeBytes"`
	AfterBytes  int64               `json:"afterBytes"`
	FreedBytes  int64               `json:"freedBytes"`
	PurgedCount int                 `json:"purgedCount"`
	FailedCount int                 `json:"failedCount,omitempty"`
	Items       []QuotaEvictionItem `json:"items,omitempty"`
	Results     []GraveResult       `json:"results,omitempty"`
}

func (e QuotaEviction) summary() *QuotaEviction {
	cp := e
	cp.Results = nil
	return &cp
}

func isLocalDiskBurial(b Burial) bool { return b.Kind == "" }

func isNonFollowedEntry(info os.FileInfo) bool {
	return info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || isReparse(info)
}

// measureLocalSize computes the exact disk payload bytes of a local file or
// directory without ever following symlinks or Windows reparse/junction points.
func measureLocalSize(path string) (int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	if isNonFollowedEntry(info) {
		return 0, nil
	}
	if !info.IsDir() {
		if info.Mode().IsRegular() {
			return info.Size(), nil
		}
		return 0, nil
	}
	return measureDirSize(path, false)
}

func measureDirSize(root string, ignoreErrors bool) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if ignoreErrors || errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if p == root {
			return nil
		}
		size, skip, err := measureDirEntry(d, ignoreErrors)
		if err != nil {
			return err
		}
		total += size
		if skip {
			return filepath.SkipDir
		}
		return nil
	})
	return total, err
}

func measureDirEntry(d fs.DirEntry, ignoreErrors bool) (size int64, skipDir bool, err error) {
	info, err := d.Info()
	if err != nil {
		if ignoreErrors || errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if isNonFollowedEntry(info) {
		return 0, d.IsDir(), nil
	}
	if info.Mode().IsRegular() {
		return info.Size(), false, nil
	}
	return 0, false, nil
}

func effectiveBurialBytes(b Burial) int64 {
	if !isLocalDiskBurial(b) {
		return 0
	}
	if b.Size > 0 {
		return b.Size
	}
	if b.Grave != "" {
		if n, err := measureLocalSize(b.Grave); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// GraveyardDiskUsage returns the count and total byte size of local disk
// burials currently stored in the graveyard, excluding non-local container items.
func (s *Service) GraveyardDiskUsage() (count int, total int64, err error) {
	st, err := s.Store.ReadSnapshot()
	if err != nil {
		return 0, 0, err
	}
	for _, b := range st.Burials {
		if !isLocalDiskBurial(b) {
			continue
		}
		count++
		total += effectiveBurialBytes(b)
	}
	return count, total, nil
}

func quotaProposalRule(itemBytes, maxBytes int64) string {
	return fmt.Sprintf("单件体积 %s 超出墓地容量上限 %s，未移入墓地；请选择永久删除、延长、不再跟踪，或调高墓地容量上限",
		textutil.Bytes(itemBytes), textutil.Bytes(maxBytes))
}

func quotaOverflowDetail(path string, itemBytes, maxBytes int64) string {
	return fmt.Sprintf("%s (%s) exceeds graveyard capacity limit %s; left in place and moved to pending decisions without evicting existing graves",
		path, textutil.Bytes(itemBytes), textutil.Bytes(maxBytes))
}

// checkBurialQuota enforces Plan A before an object enters the graveyard.
// Oversized objects are left safely in place and routed to pending decisions.
func (s *Service) checkBurialQuota(out Outcome, dryRun bool, reason string) (Outcome, bool) {
	if s.GraveyardMaxBytes <= 0 || out.Item.Kind != "" || (s.recycleMode() && !out.Item.IsDir) {
		return out, false
	}
	itemSize, err := measureLocalSize(out.Path)
	if err != nil {
		return failedMove(out, err), true
	}
	if itemSize <= s.GraveyardMaxBytes {
		return out, false
	}
	overflow := &QuotaOverflow{
		Path:      out.Path,
		ItemBytes: itemSize,
		MaxBytes:  s.GraveyardMaxBytes,
	}
	out.Action = ActionQuotaExceeded
	out.Size = itemSize
	out.Detail = quotaOverflowDetail(out.Path, itemSize, s.GraveyardMaxBytes)
	out.QuotaOverflow = overflow
	if dryRun {
		return out, true
	}
	proposalID, err := s.recordQuotaOverflow(out.Item, out.Path, itemSize, reason)
	if err != nil {
		return ledgerOutcome(out, err), true
	}
	overflow.ProposalID = proposalID
	return out, true
}

func (s *Service) recordQuotaOverflow(item Item, path string, itemBytes int64, reason string) (int64, error) {
	cleanPath := filepath.Clean(path)
	if longPath, _, err := s.FS.LongPath(path); err == nil {
		cleanPath = filepath.Clean(longPath)
	}
	var proposalID int64
	err := s.Store.UpdateHot(func(st *State) error {
		if err := requireAuditSpace(st); err != nil {
			return err
		}
		if reason == ReasonExpired {
			if _, err := s.burialRevision(st, item, reason); err != nil {
				return err
			}
		}
		proposalID = s.upsertQuotaProposal(st, item, cleanPath, itemBytes, reason)
		auditID := item.ID
		if st.Index(item.ID) < 0 {
			auditID = proposalID
		}
		detail := fmt.Sprintf("graveyard_capacity_exceeded: size=%d limit=%d proposal=#%d; left in place for user decision",
			itemBytes, s.GraveyardMaxBytes, proposalID)
		queueAudit(st, Event{Time: s.now(), Action: ActionQuotaExceeded, ID: auditID, Path: cleanPath, Detail: detail})
		return nil
	})
	return proposalID, err
}

func (s *Service) upsertQuotaProposal(st *State, item Item, cleanPath string, itemBytes int64, reason string) int64 {
	trackedID := int64(0)
	if st.Index(item.ID) >= 0 {
		trackedID = item.ID
	}
	review := &QuotaBurialReview{
		ItemID:    trackedID,
		ItemRev:   item.Rev,
		Identity:  *item.Identity,
		ItemBytes: itemBytes,
		MaxBytes:  s.GraveyardMaxBytes,
		Reason:    reason,
		IsDir:     item.IsDir,
	}
	cand := Candidate{
		Path:     cleanPath,
		Lifespan: "quota",
		Rule:     quotaProposalRule(itemBytes, s.GraveyardMaxBytes),
		Size:     itemBytes,
		Reason:   "graveyard_capacity_exceeded",
	}
	for i := range st.Proposals {
		p := &st.Proposals[i]
		if !matchesQuotaProposal(*p, trackedID, item.Identity) {
			continue
		}
		p.CandidateSeq++
		cand.Rev = p.CandidateSeq
		p.Project = item.Project
		p.QuotaReview = review
		p.Candidates = []Candidate{cand}
		return p.ID
	}
	p := Proposal{
		ID:           st.NextID,
		Kind:         ProposalGraveyardQuota,
		Project:      item.Project,
		CreatedAt:    s.now(),
		CandidateSeq: 1,
		Candidates:   []Candidate{{Path: cand.Path, Lifespan: cand.Lifespan, Rule: cand.Rule, Size: cand.Size, Reason: cand.Reason, Rev: 1}},
		QuotaReview:  review,
	}
	st.NextID++
	st.Proposals = append(st.Proposals, p)
	return p.ID
}

func matchesQuotaProposal(p Proposal, trackedID int64, id *Identity) bool {
	if p.Kind != ProposalGraveyardQuota || p.QuotaReview == nil {
		return false
	}
	if trackedID > 0 && p.QuotaReview.ItemID == trackedID {
		return true
	}
	return id != nil && p.QuotaReview.Identity == *id
}

func awaitingQuotaDecision(st *State, it Item, maxBytes int64) bool {
	return ledger.AwaitingQuotaDecision(st, it, maxBytes)
}

func dropQuotaProposal(st *State, id int64, identity *Identity) {
	st.Proposals = slices.DeleteFunc(st.Proposals, func(p Proposal) bool {
		return matchesQuotaProposal(p, id, identity)
	})
}

// PreviewGraveyardQuotaEviction calculates which oldest local graveyard items
// would be evicted under maxBytes without modifying any files or ledger state.
func (s *Service) PreviewGraveyardQuotaEviction(maxBytes int64) (QuotaEviction, error) {
	var ev QuotaEviction
	err := s.withGraveyardLock(func() error {
		ro := *s
		ro.readOnlyOwners = true
		var innerErr error
		ev, innerErr = ro.enforceGraveyardQuotaLocked(true, maxBytes)
		return innerErr
	})
	return ev, err
}

// EnforceGraveyardQuota purges the oldest local disk burials in strict FIFO
// order (buried_at ASC, id ASC) until usage fits within s.GraveyardMaxBytes.
func (s *Service) EnforceGraveyardQuota(dryRun bool) (ev QuotaEviction, finalErr error) {
	if s.GraveyardMaxBytes <= 0 {
		return QuotaEviction{}, nil
	}
	if !dryRun {
		var notify func()
		s, notify = s.trackChanges()
		defer notify()
	}
	s, flushAudit := s.auditBatch(dryRun)
	defer func() { finalErr = errors.Join(finalErr, flushAudit()) }()
	err := s.withGraveyardLock(func() error {
		var innerErr error
		ev, innerErr = s.enforceGraveyardQuotaLocked(dryRun, s.GraveyardMaxBytes)
		return innerErr
	})
	return ev, err
}

func (s *Service) enforceGraveyardQuotaLocked(dryRun bool, maxBytes int64) (QuotaEviction, error) {
	state, err := s.readQuotaState(dryRun)
	if err != nil {
		return QuotaEviction{}, err
	}
	candidates, beforeBytes := sortedQuotaCandidates(state.Burials)
	ev := QuotaEviction{MaxBytes: maxBytes, BeforeBytes: beforeBytes, AfterBytes: beforeBytes}
	if maxBytes <= 0 || beforeBytes <= maxBytes {
		return ev, nil
	}
	pending, _ := s.scanManifests(state)
	blocked := s.blockedPurges(state, pending)
	remaining := beforeBytes
	var auditErr error
	for _, b := range candidates {
		if remaining <= maxBytes {
			break
		}
		sz := effectiveBurialBytes(b)
		r, fatalErr, itemAuditErr := s.evictOneQuotaBurial(b, state, blocked[b.ID], dryRun)
		auditErr = errors.Join(auditErr, itemAuditErr)
		if r.Error != "" || fatalErr != nil {
			ev.FailedCount++
			ev.Results = append(ev.Results, r)
			if fatalErr != nil {
				ev.AfterBytes = max(int64(0), remaining)
				return ev, errors.Join(fatalErr, auditErr)
			}
			continue
		}
		if r.Action == ActionPurged || r.Action == ActionLost {
			remaining -= sz
			ev.FreedBytes += sz
			ev.PurgedCount++
			ev.Items = append(ev.Items, QuotaEvictionItem{ID: b.ID, Path: b.Path, Size: sz, BuriedAt: b.BuriedAt})
			ev.Results = append(ev.Results, r)
		}
	}
	ev.AfterBytes = max(int64(0), remaining)
	if len(ev.Results) > 0 {
		ev.Results[len(ev.Results)-1].QuotaEviction = ev.summary()
	}
	return ev, auditErr
}

func (s *Service) readQuotaState(dryRun bool) (State, error) {
	if dryRun {
		return s.Store.ReadSnapshot()
	}
	return s.Store.Read()
}

func sortedQuotaCandidates(burials []Burial) ([]Burial, int64) {
	candidates := make([]Burial, 0, len(burials))
	var total int64
	for _, b := range burials {
		if !isLocalDiskBurial(b) {
			continue
		}
		candidates = append(candidates, b)
		total += effectiveBurialBytes(b)
	}
	slices.SortFunc(candidates, func(a, b Burial) int {
		if c := a.BuriedAt.Compare(b.BuriedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return candidates, total
}

func (s *Service) evictOneQuotaBurial(b Burial, state State, blocked string, dryRun bool) (GraveResult, error, error) {
	if blocked != "" {
		r, err := s.blockedPurge(b, blocked, dryRun)
		r.Detail = PurgeBasisQuotaFIFO
		return r, err, nil
	}
	r := GraveResult{Ref: fmt.Sprintf("#%d", b.ID), Burial: &b, Detail: PurgeBasisQuotaFIFO}
	if err := s.requirePurgeAuditSpace(b, dryRun); err != nil {
		r.Error = err.Error()
		return r, err, nil
	}
	observed := s.observePurge(b, false)
	decision := decidePurge(b, false, observed)
	err := s.executePurge(&r, state, decision, PurgeBasisQuotaFIFO, dryRun)
	if commitFailed(err) {
		r.Error = err.Error()
		return r, err, nil
	}
	retryErr := s.retryPurgeResult(&r, dryRun)
	return r, retryErr, err
}

func (s *Service) purgeQuotaLocked(run *purgeRun) error {
	ev, err := s.enforceGraveyardQuotaLocked(run.dryRun, s.GraveyardMaxBytes)
	run.results = append(run.results, ev.Results...)
	if commitFailed(err) {
		return err
	}
	run.auditErr = errors.Join(run.auditErr, err)
	return nil
}

func (s *Service) applyBuryQuota(results []GraveResult) error {
	if s.GraveyardMaxBytes <= 0 {
		return nil
	}
	lastBuried := -1
	for i, r := range results {
		if r.Action == ActionBuried {
			lastBuried = i
		}
	}
	if lastBuried < 0 {
		return nil
	}
	ev, err := s.EnforceGraveyardQuota(false)
	if ev.PurgedCount > 0 || ev.FailedCount > 0 {
		results[lastBuried].QuotaEviction = ev.summary()
	}
	return err
}

func (s *Service) applySweepQuota(outcomes []Outcome, dryRun bool) ([]Outcome, error) {
	if s.GraveyardMaxBytes <= 0 {
		return outcomes, nil
	}
	var (
		ev  QuotaEviction
		err error
	)
	if dryRun {
		ev, err = s.PreviewGraveyardQuotaEviction(s.GraveyardMaxBytes)
	} else {
		ev, err = s.EnforceGraveyardQuota(false)
	}
	if ev.PurgedCount == 0 && ev.FailedCount == 0 {
		return outcomes, err
	}
	summary := ev.summary()
	for i := len(outcomes) - 1; i >= 0; i-- {
		if outcomes[i].Action == ActionBuried {
			outcomes[i].QuotaEviction = summary
			return outcomes, err
		}
	}
	for i, item := range ev.Items {
		out := Outcome{
			Item:   Item{ID: item.ID, Path: item.Path},
			Action: ActionPurged,
			Path:   item.Path,
			Size:   item.Size,
			Detail: PurgeBasisQuotaFIFO,
		}
		if i == len(ev.Items)-1 {
			out.QuotaEviction = summary
		}
		outcomes = append(outcomes, out)
	}
	return outcomes, err
}

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
			path, textutil.Bytes(size), textutil.Bytes(s.GraveyardMaxBytes))
	}
	results, err := s.Bury([]string{path}, s.quotaTrackedForce(p), "approved quota decision")
	if err != nil && !OnlyAuditErrors(err) {
		return Outcome{Path: path}, err
	}
	if len(results) == 0 {
		return Outcome{Path: path}, errors.New("no burial result returned")
	}
	r := results[0]
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
	path := p.Candidates[0].Path
	if err := s.verifyQuotaLiveObject(path, p.QuotaReview.Identity); err != nil {
		return quotaProposalErr(r, err), err
	}
	buried, err := s.buryQuotaOnly(path, s.quotaTrackedForce(p))
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
		r.Error = err.Error()
	} else if Failed(results) {
		r.Error = results[0].Error
	}
	return r, err
}

func (s *Service) extendQuotaTarget(p Proposal, d time.Duration) ([]Result, error) {
	st, err := s.Store.ReadHot()
	if err != nil {
		return nil, err
	}
	if p.QuotaReview.ItemID > 0 {
		idx := st.Index(p.QuotaReview.ItemID)
		if idx >= 0 && st.Items[idx].Identity != nil && *st.Items[idx].Identity == p.QuotaReview.Identity {
			return s.Extend([]string{fmt.Sprint(p.QuotaReview.ItemID)}, d)
		}
	}
	results, err := s.Add([]string{p.Candidates[0].Path}, AddOptions{
		ExpiresAt: s.now().Add(d),
		Source:    "cli",
		Project:   p.Project,
		Note:      fmt.Sprintf("extended from quota decision #%d", p.ID),
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
		return fmt.Errorf("%s changed since review: %w", path, ErrUnsafe)
	}
	return nil
}

func (s *Service) quotaTrackedForce(p Proposal) bool {
	if p.QuotaReview == nil || p.QuotaReview.ItemID == 0 {
		return false
	}
	st, err := s.Store.ReadHot()
	if err != nil {
		return false
	}
	idx := st.Index(p.QuotaReview.ItemID)
	return idx >= 0 && st.Items[idx].Force
}

func (s *Service) buryQuotaOnly(path string, force bool) (*Burial, error) {
	tx := *s
	tx.GraveyardMaxBytes = 0
	tx.Mode = ModeGraveyard
	results, err := tx.bury([]string{path}, force, "quota_exceeded_permanent_delete", "quota-decision")
	if err != nil && !OnlyAuditErrors(err) {
		return nil, err
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
