package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ExistingProposals is a read-only display view. Only definite absence is
// filtered; permission errors and broken symlinks remain visible. No Git here:
// session hooks share a one-second budget and background work vets tracking.
func ExistingProposals(proposals []Proposal) []Proposal {
	out := make([]Proposal, 0, len(proposals))
	for _, p := range proposals {
		candidates := make([]Candidate, 0, len(p.Candidates))
		for _, c := range p.Candidates {
			if c.Kind == KindContainer {
				candidates = append(candidates, c)
				continue
			}
			if _, err := os.Lstat(c.Path); !errors.Is(err, os.ErrNotExist) {
				candidates = append(candidates, c)
			}
		}
		if len(candidates) > 0 {
			p.Candidates = candidates
			out = append(out, p)
		}
	}
	return out
}

// PruneProposal removes stale snapshot revisions, serializing with user
// decisions. Automatic withdrawal neither grants consent nor remembers a
// rejection; its distinct audit event records why the proposal disappeared.
func (s *Service) PruneProposal(p Proposal, stale []Candidate) error {
	return s.pruneProposal(p, stale, "candidate no longer exists or is Git-tracked", nil)
}

// PruneDeletedProposal checks deletion again while serializing with decisions.
func (s *Service) PruneDeletedProposal(p Proposal) error {
	if p.Kind == ProposalWatch || p.Kind == ProposalWatchBurial || p.Kind == ProposalContainer {
		return fmt.Errorf("proposal kind %q does not use project deletion evidence", p.Kind)
	}
	return s.pruneProposal(p, p.Candidates, "project deleted", func() error {
		deleted, err := s.ProposalProjectDeleted(p.Project)
		if err != nil {
			return err
		}
		if !deleted {
			return fmt.Errorf("proposal project %q is available again; retry later", p.Project)
		}
		return nil
	})
}

func (s *Service) pruneProposal(p Proposal, stale []Candidate, reason string, check func() error) error {
	if len(stale) == 0 {
		return nil
	}
	release, err := s.lockProposals()
	if err != nil {
		return err
	}
	defer release()
	if check != nil {
		if err := check(); err != nil {
			return err
		}
	}
	revisions := make(map[string]uint64, len(stale))
	for _, c := range stale {
		revisions[c.Path] = c.Rev
	}
	var events []Event
	err = s.Store.Update(func(st *State) error {
		idx := st.ProposalIndex(p.ID)
		if idx < 0 {
			return nil
		}
		current := &st.Proposals[idx]
		if current.Project != p.Project || current.Kind != p.Kind {
			return nil
		}
		current.Candidates = slices.DeleteFunc(current.Candidates, func(c Candidate) bool {
			rev, ok := revisions[c.Path]
			if !ok || rev != c.Rev {
				return false
			}
			events = append(events, Event{Time: s.now(), Action: "proposal-pruned", ID: p.ID, Path: c.Path, Detail: reason})
			return true
		})
		if len(current.Candidates) == 0 {
			st.Proposals = append(st.Proposals[:idx], st.Proposals[idx+1:]...)
			events = append(events, Event{Time: s.now(), Action: "proposal-withdrawn", ID: p.ID, Path: p.Project, Detail: "automatically withdrawn: no candidates remain; " + reason})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.Store.RecordCommitted(events...)
}

// Consent returns a project's consent state ("" when it was never asked).
func (s *Service) Consent(root string) (string, error) {
	state, err := s.Store.Read()
	if err != nil {
		return "", err
	}
	return state.Projects[ProjectKey(root)].Consent, nil
}

// Propose records candidates for approval, merging them into the open
// proposal of the same kind for the project. created is true when the
// proposal is new; added counts candidates it did not hold yet. Repeated
// paths refresh the candidate and its revision, even while it is claimed.
func (s *Service) Propose(kind, project, session string, candidates []Candidate) (Proposal, bool, int, error) {
	return s.ProposeObserved(kind, project, session, candidates, nil)
}

// ProposeObserved ties observed candidates to their originating object in
// the same transaction that saves them. Ordinary proposals pass nil.
func (s *Service) ProposeObserved(kind, project, session string, candidates []Candidate, observed *Observation) (Proposal, bool, int, error) {
	var (
		out     Proposal
		created bool
		added   int
	)
	if project != "" {
		root, _, err := s.FS.LongPath(project)
		if err != nil {
			return out, false, 0, err
		}
		project = filepath.Clean(root)
	}
	candidates = slices.Clone(candidates)
	for i := range candidates {
		if candidates[i].Kind == KindContainer {
			continue
		}
		path, _, err := s.FS.LongPath(candidates[i].Path)
		if err != nil {
			return out, false, 0, err
		}
		candidates[i].Path = filepath.Clean(path)
	}
	err := s.Store.Update(func(st *State) error {
		if err := s.CheckObservation(observed); err != nil {
			return err
		}
		if st.ObservationCompleted(observed) {
			return nil
		}
		if kind == ProposalLeftovers {
			candidates = slices.DeleteFunc(candidates, func(c Candidate) bool {
				return st.LeftoverPathRejected(project, c.Path) || s.leftoverUnavailable(st, project, session, c)
			})
			if len(candidates) == 0 {
				return nil
			}
		}
		idx := -1
		for i, p := range st.Proposals {
			if p.Kind == kind && samePath(p.Project, project) && ((kind != ProposalLeftovers && kind != ProposalContainer) || p.Session == session) {
				idx = i
			}
		}
		if idx < 0 {
			st.Proposals = append(st.Proposals, Proposal{ID: st.NextID, Kind: kind, Project: project, Session: session, CreatedAt: s.now()})
			st.NextID++
			idx, created = len(st.Proposals)-1, true
		}
		p := &st.Proposals[idx]
		for _, c := range p.Candidates {
			p.CandidateSeq = max(p.CandidateSeq, c.Rev)
		}
		for _, c := range candidates {
			if (kind == ProposalLeftovers || kind == ProposalContainer) && st.LeftoverRejected(project, session, candidateKey(c)) {
				continue
			}
			dup := false
			for i, have := range p.Candidates {
				if sameCandidate(have, c) {
					p.CandidateSeq++
					c.Rev = p.CandidateSeq
					p.Candidates[i] = c
					dup = true
					break
				}
			}
			if !dup {
				p.CandidateSeq++
				c.Rev = p.CandidateSeq
				p.Candidates = append(p.Candidates, c)
				added++
			}
		}
		if created && added == 0 {
			st.Proposals = st.Proposals[:idx]
			created = false
			return nil
		}
		out = *p
		st.RememberObservation(observed, s.now())
		return nil
	})
	return out, created, added, err
}

// MarkNotified records that a session was told about a proposal and reports
// whether it had been told before.
func (s *Service) MarkNotified(id int64, session string) (bool, error) {
	before, err := s.MarkNotifiedBatch([]int64{id}, session)
	return before[0], err
}

// MarkNotifiedBatch acknowledges a fully prepared context in one transaction;
// a later lock timeout cannot leave a prefix acknowledged but never delivered.
func (s *Service) MarkNotifiedBatch(ids []int64, session string) ([]bool, error) {
	before := make([]bool, len(ids))
	err := s.Store.Update(func(st *State) error {
		for i, id := range ids {
			idx := st.ProposalIndex(id)
			if idx < 0 {
				before[i] = true // no longer pending
				continue
			}
			before[i] = slices.Contains(st.Proposals[idx].Notified, session)
			if !before[i] {
				st.Proposals[idx].Notified = append(st.Proposals[idx].Notified, session)
			}
		}
		return nil
	})
	return before, err
}

// ProposalResult is the outcome of approving or rejecting one proposal.
type ProposalResult struct {
	Ref      string   `json:"ref"`
	Proposal Proposal `json:"proposal"`
	Results  []Result `json:"results,omitempty"` // one per candidate on approval
	Error    string   `json:"error,omitempty"`
}

func parseProposalRef(ref string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a proposal number", ref)
	}
	return id, nil
}

// lockProposals serializes approvals and rejections while leaving candidates
// durable until their outcomes are saved. The store lock still guards writes.
func (s *Service) lockProposals() (func(), error) {
	if err := os.MkdirAll(s.Store.Home, 0o700); err != nil {
		return nil, err
	}
	release, ok, err := TryLockExclusive(filepath.Join(s.Store.Home, "proposals.lock"))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("another proposal decision is in progress")
	}
	return release, nil
}

// ClaimProposal reserves a pending proposal without deleting it. The caller
// must ResolveProposal and then release the claim. A failed write or a killed
// process leaves the candidates pending under their original ID. Claiming an
// onboard proposal permits automatic registration in its project from now on.
func (s *Service) ClaimProposal(ref string) (Proposal, func(), error) {
	return s.claimProposal(ref, nil)
}

func (s *Service) claimProposal(ref string, review func(Proposal) error) (Proposal, func(), error) {
	id, err := parseProposalRef(ref)
	if err != nil {
		return Proposal{}, nil, err
	}
	release, err := s.lockProposals()
	if err != nil {
		return Proposal{}, nil, err
	}
	var p Proposal
	err = s.Store.Update(func(st *State) error {
		idx := st.ProposalIndex(id)
		if idx < 0 {
			return fmt.Errorf("#%d is not a pending proposal", id)
		}
		p = st.Proposals[idx]
		if review != nil {
			if err := review(p); err != nil {
				return err
			}
		}
		if p.Kind == ProposalOnboard {
			if st.Projects == nil {
				st.Projects = map[string]ProjectState{}
			}
			st.Projects[ProjectKey(p.Project)] = ProjectState{Consent: ConsentApproved, At: s.now()}
		}
		return nil
	})
	if err != nil {
		release()
		return Proposal{}, nil, err
	}
	return p, release, nil
}

// ResolveProposal removes only resolved candidates from the claimed snapshot.
// Retryable candidates and anything proposed concurrently retain the same ID.
// The caller must hold the claim until this write finishes (or fails).
func (s *Service) ResolveProposal(p Proposal, pending []Candidate) error {
	resolved := make(map[string]uint64, len(p.Candidates))
	for _, c := range p.Candidates {
		resolved[candidateKey(c)] = c.Rev
	}
	for _, c := range pending {
		delete(resolved, candidateKey(c))
	}
	return s.Store.Update(func(st *State) error {
		idx := st.ProposalIndex(p.ID)
		if idx < 0 {
			return fmt.Errorf("#%d is no longer pending", p.ID)
		}
		current := &st.Proposals[idx]
		var settled []Candidate
		current.Candidates = slices.DeleteFunc(current.Candidates, func(c Candidate) bool {
			rev, ok := resolved[candidateKey(c)]
			if ok && rev == c.Rev {
				settled = append(settled, c)
				return true
			}
			return false
		})
		if len(current.Candidates) == 0 {
			st.Proposals = append(st.Proposals[:idx], st.Proposals[idx+1:]...)
		}
		st.removeLeftoverTwins(p, settled)
		return nil
	})
}

// Reject drops proposals. Rejecting an onboard proposal marks its project as
// declined, so Obituary stops proposing automatic registration there.
func (s *Service) Reject(refs []string) ([]ProposalResult, error) {
	return s.reject(refs, nil)
}

func (s *Service) reject(refs []string, review func(int, Proposal) error) ([]ProposalResult, error) {
	release, err := s.lockProposals()
	if err != nil {
		return nil, err
	}
	defer release()
	var out []ProposalResult
	for i, ref := range refs {
		r := ProposalResult{Ref: ref}
		id, err := parseProposalRef(ref)
		if err != nil {
			r.Error = err.Error()
			out = append(out, r)
			continue
		}
		err = s.Store.Update(func(st *State) error {
			idx := st.ProposalIndex(id)
			if idx < 0 {
				r.Error = fmt.Sprintf("#%d is not a pending proposal", id)
				return nil
			}
			r.Proposal = st.Proposals[idx]
			if r.Proposal.Kind == ProposalWatchBurial {
				if _, err := watchBurialItem(st, r.Proposal); err != nil {
					r.Error = err.Error()
					return nil
				}
			}
			if review != nil {
				if err := review(i, r.Proposal); err != nil {
					r.Error = err.Error()
					return nil
				}
			}
			for i := range st.Watches {
				if r.Proposal.Kind == ProposalWatch && st.Watches[i].ID == r.Proposal.WatchID {
					st.Watches[i].Consent = ConsentDeclined
				}
			}
			if r.Proposal.Kind == ProposalWatchBurial {
				st.rejectWatchBurial(r.Proposal, s.now())
			}
			if r.Proposal.Kind == ProposalLeftovers || r.Proposal.Kind == ProposalContainer {
				st.rememberRejectedLeftovers(r.Proposal, s.now())
			}
			st.Proposals = append(st.Proposals[:idx], st.Proposals[idx+1:]...)
			st.removeLeftoverTwins(r.Proposal, r.Proposal.Candidates)
			if r.Proposal.Kind == ProposalOnboard {
				if st.Projects == nil {
					st.Projects = map[string]ProjectState{}
				}
				st.Projects[ProjectKey(r.Proposal.Project)] = ProjectState{Consent: ConsentDeclined, At: s.now()}
			}
			return nil
		})
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

const (
	maxRejectedSessions = 256
	maxRejectedPaths    = 4096
)

// LeftoverRejected also excludes collapsed directories containing a rejected
// path and children of rejected directories, independent of Git's spelling.
func (st *State) LeftoverRejected(project, session, path string) bool {
	for _, r := range st.LeftoverRejections {
		if !samePath(r.Project, project) || r.Session != session {
			continue
		}
		for _, p := range r.Paths {
			if samePath(path, p) || isInside(path, p) || isInside(p, path) {
				return true
			}
		}
	}
	return false
}

func (st *State) rememberRejectedLeftovers(p Proposal, now time.Time) {
	idx := -1
	for i, r := range st.LeftoverRejections {
		if samePath(r.Project, p.Project) && r.Session == p.Session {
			idx = i
		}
	}
	if idx < 0 {
		idx = len(st.LeftoverRejections)
		st.LeftoverRejections = append(st.LeftoverRejections, LeftoverRejection{Project: ProjectKey(p.Project), Session: p.Session})
	}
	r := &st.LeftoverRejections[idx]
	r.At = now
	for _, c := range p.Candidates {
		if slices.ContainsFunc(r.Paths, func(path string) bool { return samePath(path, candidateKey(c)) }) {
			continue
		}
		r.Paths = append(r.Paths, candidateKey(c))
	}
	// Evict whole sessions by last rejection time, keeping the current
	// decision newest even when the clock ties. Within it paths are FIFO.
	current := *r
	st.LeftoverRejections = append(st.LeftoverRejections[:idx], st.LeftoverRejections[idx+1:]...)
	slices.SortStableFunc(st.LeftoverRejections, func(a, b LeftoverRejection) int { return a.At.Compare(b.At) })
	if len(current.Paths) > maxRejectedPaths {
		current.Paths = current.Paths[len(current.Paths)-maxRejectedPaths:]
	}
	st.LeftoverRejections = append(st.LeftoverRejections, current)
	total := 0
	for _, r := range st.LeftoverRejections {
		total += len(r.Paths)
	}
	for len(st.LeftoverRejections) > maxRejectedSessions || total > maxRejectedPaths {
		total -= len(st.LeftoverRejections[0].Paths)
		st.LeftoverRejections = st.LeftoverRejections[1:]
	}
}

// DueWithin lists a project's items expiring before now+window.
func (s *Service) DueWithin(project string, window time.Duration) ([]Item, error) {
	state, err := s.Store.Read()
	if err != nil {
		return nil, err
	}
	limit := s.now().Add(window)
	var out []Item
	for _, it := range state.Items {
		// Explicit registrations (obit add, MCP, Explorer) carry no project;
		// they belong to the project whose tree contains them. Scratch
		// directories from obit mktemp are not worth announcing.
		inProject := strings.EqualFold(it.Project, project) || it.Project == "" && isInside(it.Path, project)
		if inProject && it.Source != "mktemp" && it.ExpiresAt.Before(limit) {
			out = append(out, it)
		}
	}
	return out, nil
}
