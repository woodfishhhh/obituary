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
	MaxBytes      int64               `json:"maxBytes"`
	BeforeBytes   int64               `json:"beforeBytes"`
	AfterBytes    int64               `json:"afterBytes"`
	FreedBytes    int64               `json:"freedBytes"`
	PurgedCount   int                 `json:"purgedCount"`
	FailedCount   int                 `json:"failedCount,omitempty"`
	Items         []QuotaEvictionItem `json:"items,omitempty"`
	Results       []GraveResult       `json:"results,omitempty"`
	GraveyardHash string              `json:"-"`
}

func (e QuotaEviction) summary() *QuotaEviction {
	cp := e
	cp.Results = nil
	return &cp
}

func isQuotaManagedBurial(b Burial) bool { return b.IsFile() && b.Source != "quota-decision" }

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
	if !isQuotaManagedBurial(b) {
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
// burials currently stored in the graveyard, excluding non-local container
// items and temporary permanent-delete graves.
func (s *Service) GraveyardDiskUsage() (count int, total int64, err error) {
	st, err := s.Store.Inspect()
	if err != nil {
		return 0, 0, err
	}
	for _, b := range st.Burials {
		if !isQuotaManagedBurial(b) {
			continue
		}
		count++
		total += effectiveBurialBytes(b)
	}
	return count, total, nil
}

// GraveyardSnapshotHash returns the current committed graveyard snapshot hash
// without acquiring locks or creating directories.
func (s *Service) GraveyardSnapshotHash() (string, error) {
	st, err := s.Store.Inspect()
	if err != nil {
		return "", err
	}
	return graveyardHash(st), nil
}

func graveyardHash(st State) string {
	if st.Graveyard == nil {
		return ""
	}
	return st.Graveyard.Hash
}

// formatQuotaBytes formats binary byte sizes identically to textutil.Bytes
// without importing internal/textutil, which would form a POSIX test import
// cycle via internal/testenv -> internal/runkey (runkey_unix.go) -> internal/core.
func formatQuotaBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func quotaProposalRule(itemBytes, maxBytes int64) string {
	return fmt.Sprintf("单件体积 %s 超出墓地容量上限 %s，未移入墓地；请选择永久删除、延长、不再跟踪，或调高墓地容量上限",
		formatQuotaBytes(itemBytes), formatQuotaBytes(maxBytes))
}

func quotaOverflowDetail(path string, itemBytes, maxBytes int64) string {
	return fmt.Sprintf("%s (%s) exceeds graveyard capacity limit %s; left in place and moved to pending decisions without evicting existing graves; ask the user to decide in Obituary or raise graveyard.max_size; do not bypass Obituary with raw rm",
		path, formatQuotaBytes(itemBytes), formatQuotaBytes(maxBytes))
}

// checkBurialQuota enforces Plan A before an object enters the graveyard.
// Oversized objects are left safely in place and routed to pending decisions.
func (s *Service) checkBurialQuota(out Outcome, dryRun bool, reason string) (Outcome, bool) {
	if s.GraveyardMaxBytes <= 0 || out.Item.IsExternal() || (s.recycleMode() && !out.Item.IsDir) {
		return out, false
	}
	itemSize, err := measureLocalSize(out.Path)
	if err != nil {
		return failedMove(out, err), true
	}
	out.Size = itemSize
	if itemSize <= s.GraveyardMaxBytes {
		return out, false
	}
	overflow := &QuotaOverflow{
		Path:      out.Path,
		ItemBytes: itemSize,
		MaxBytes:  s.GraveyardMaxBytes,
	}
	out.Action = ActionQuotaExceeded
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
		dropWatchBurial(st, item.ID)
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
		ItemID: trackedID, ItemRev: item.Rev, Identity: *item.Identity,
		ItemBytes: itemBytes, MaxBytes: s.GraveyardMaxBytes, Reason: reason, IsDir: item.IsDir,
	}
	cand := Candidate{
		Path: cleanPath, Lifespan: "quota", Rule: quotaProposalRule(itemBytes, s.GraveyardMaxBytes),
		Size: itemBytes, Reason: "graveyard_capacity_exceeded",
	}
	watchID, watchBurial := s.quotaWatchReview()
	for i := range st.Proposals {
		p := &st.Proposals[i]
		if !matchesQuotaProposal(*p, trackedID, item.Identity) {
			continue
		}
		p.CandidateSeq++
		cand.Rev = p.CandidateSeq
		p.Project, p.QuotaReview, p.Candidates = item.Project, review, []Candidate{cand}
		if watchBurial != nil {
			p.WatchID, p.WatchBurial = watchID, watchBurial
		}
		return p.ID
	}
	cand.Rev = 1
	p := Proposal{
		ID: st.NextID, Kind: ProposalGraveyardQuota, Project: item.Project, CreatedAt: s.now(),
		CandidateSeq: 1, Candidates: []Candidate{cand},
		WatchID: watchID, WatchBurial: watchBurial, QuotaReview: review,
	}
	st.NextID++
	st.Proposals = append(st.Proposals, p)
	return p.ID
}

func (s *Service) quotaWatchReview() (int64, *WatchBurialReview) {
	if s.approvedWatch == nil {
		return 0, nil
	}
	return s.approvedWatch.WatchID, s.approvedWatch.WatchBurial
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
// would be evicted under maxBytes without acquiring locks or creating files.
func (s *Service) PreviewGraveyardQuotaEviction(maxBytes int64) (QuotaEviction, error) {
	state, err := s.Store.Inspect()
	if err != nil {
		return QuotaEviction{}, err
	}
	ro := *s
	ro.readOnlyOwners = true
	return ro.enforceGraveyardQuotaOnState(state, true, maxBytes)
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
	return s.enforceGraveyardQuotaOnState(state, dryRun, maxBytes)
}

func (s *Service) enforceGraveyardQuotaOnState(state State, dryRun bool, maxBytes int64) (QuotaEviction, error) {
	candidates, beforeBytes := sortedQuotaCandidates(state.Burials)
	ev := QuotaEviction{MaxBytes: maxBytes, BeforeBytes: beforeBytes, AfterBytes: beforeBytes, GraveyardHash: graveyardHash(state)}
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
			if r.Burial != nil && r.Burial.PurgeIntent != nil {
				remaining -= sz
			}
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
			if !dryRun && ev.PurgedCount%sweepBatchSize == 0 {
				auditErr = errors.Join(auditErr, s.Store.FlushAudit())
			}
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
		return s.Store.Inspect()
	}
	return s.Store.Read()
}

func sortedQuotaCandidates(burials []Burial) ([]Burial, int64) {
	candidates := make([]Burial, 0, len(burials))
	var total int64
	for _, b := range burials {
		if !isQuotaManagedBurial(b) || b.PurgeIntent != nil {
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
	if dryRun && b.Grave == "" {
		return GraveResult{Ref: fmt.Sprintf("#%d", b.ID), Burial: &b, Action: ActionPurged, Detail: PurgeBasisQuotaFIFO}, nil, nil
	}
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
	var (
		ev  QuotaEviction
		err error
	)
	if run.dryRun {
		state, readErr := s.Store.Inspect()
		if readErr != nil {
			return readErr
		}
		ro := *s
		ro.readOnlyOwners = true
		ev, err = ro.enforceGraveyardQuotaOnState(projectPurgedBurials(state, run.results), true, s.GraveyardMaxBytes)
	} else {
		ev, err = s.enforceGraveyardQuotaLocked(false, s.GraveyardMaxBytes)
	}
	run.results = append(run.results, ev.Results...)
	if commitFailed(err) {
		return err
	}
	run.auditErr = errors.Join(run.auditErr, err)
	return nil
}

func projectPurgedBurials(state State, results []GraveResult) State {
	removed := make(map[int64]bool, len(results))
	for _, r := range results {
		if r.Burial != nil && (r.Action == ActionPurged || r.Action == ActionLost) {
			removed[r.Burial.ID] = true
		}
	}
	if len(removed) == 0 {
		return state
	}
	state.Burials = slices.DeleteFunc(slices.Clone(state.Burials), func(b Burial) bool { return removed[b.ID] })
	return state
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
	ev, err := s.runSweepQuota(outcomes, dryRun)
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
	if len(ev.Items) == 0 && ev.FailedCount > 0 {
		return appendFailedSweepQuota(outcomes, ev, summary), err
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

func (s *Service) runSweepQuota(outcomes []Outcome, dryRun bool) (QuotaEviction, error) {
	if !dryRun {
		return s.EnforceGraveyardQuota(false)
	}
	state, err := s.Store.Inspect()
	if err != nil {
		return QuotaEviction{}, err
	}
	state = projectSweepBurials(state, outcomes, s.now())
	ro := *s
	ro.readOnlyOwners = true
	return ro.enforceGraveyardQuotaOnState(state, true, s.GraveyardMaxBytes)
}

func projectSweepBurials(state State, outcomes []Outcome, now time.Time) State {
	burials := slices.Clone(state.Burials)
	for _, out := range outcomes {
		if out.Action != ActionBuried || out.Item.IsExternal() {
			continue
		}
		sz := out.Size
		if sz <= 0 {
			sz, _ = measureLocalSize(out.Path)
		}
		burials = append(burials, Burial{ID: out.Item.ID, Path: out.Path, Size: sz, BuriedAt: now})
	}
	state.Burials = burials
	return state
}

func appendFailedSweepQuota(outcomes []Outcome, ev QuotaEviction, summary *QuotaEviction) []Outcome {
	for i, r := range ev.Results {
		var id int64
		var path string
		if r.Burial != nil {
			id, path = r.Burial.ID, r.Burial.Path
		}
		out := Outcome{
			Item:   Item{ID: id, Path: path},
			Action: ActionBlocked,
			Path:   path,
			Detail: r.Error,
		}
		if i == len(ev.Results)-1 {
			out.QuotaEviction = summary
		}
		outcomes = append(outcomes, out)
	}
	return outcomes
}
