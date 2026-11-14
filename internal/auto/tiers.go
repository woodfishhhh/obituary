package auto

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
)

// The proposal mode ([proposals] mode) decides which automatic registrations
// wait for the user. "tiered" lets in, without asking, only what is
// demonstrably the session's own and recreatable: files the hook saw the agent
// write that did not exist when the session started, Git-ignored dependencies,
// build output and caches, and containers tied to the project. "auto" lets in
// everything automatic registration finds; "ask" proposes everything. Every
// path still passes vetUnit (never Git-tracked, never shortening a lifespan),
// and each registration made without asking leaves an AutoReceipt so it is
// announced afterwards and can be undone.

// directClass decides whether a rule-matched unit may be registered without
// asking, and as what ("" means propose it). A declined project never gets here.
func (e *Engine) directClass(root, unit string, opts Options) string {
	mode := e.Svc.Proposals()
	switch {
	case mode == core.ProposalsAsk:
		return ""
	case opts.Written && e.createdThisSession(root, opts.Session, unit):
		return core.AutoCreated
	case within(opts.regenerable, unit):
		return core.AutoRegenerable
	case mode == core.ProposalsAuto:
		return core.AutoAll
	}
	return ""
}

// createdThisSession is the evidence that a written unit is the agent's own,
// on top of the hook having seen the write. The session's Git snapshot, taken
// at session start, lists neither the unit, a directory above it, nor anything
// inside it; and the file system created the unit and everything in it after
// the session started. The second check catches what a path snapshot cannot:
// an older file or folder renamed or moved to a new name during the session
// keeps its creation time. Without a snapshot (no session start seen, one from
// an older version, a session outside this project) or a creation time, the
// unit is not proven created and is proposed instead.
func (e *Engine) createdThisSession(root, session, unit string) bool {
	root, err := pathutil.LongAbs(root)
	if err != nil {
		return false
	}
	snap, err := e.readSnapshot(e.snapshotPath(root, session))
	return err == nil && createdIn(snap, root, unit)
}

func createdIn(snap snapshot, root, unit string) bool {
	if !snap.Git {
		return false
	}
	u := strings.ToLower(filepath.Clean(unit))
	if !inside(u, strings.ToLower(filepath.Clean(root))) {
		return false
	}
	for _, p := range snap.Paths {
		q := strings.ToLower(filepath.Clean(p))
		if q == u || inside(u, q) || inside(q, u) {
			return false
		}
	}
	return bornAfter(unit, snap.At.Add(-creationSlack))
}

// creationSlack covers coarse file-system clocks (FAT keeps two seconds) and
// files created between Git's listing and the snapshot's timestamp.
const creationSlack = 2 * time.Second

// maxCreatedEntries bounds the walk a hook makes; a larger unit is proposed.
const maxCreatedEntries = 5000

// bornAfter reports whether path and everything below it was created after
// start. Links are not followed; an unreadable entry or creation time fails.
func bornAfter(path string, start time.Time) bool {
	entries := 0
	err := filepath.WalkDir(path, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries++
		if born, ok := birthTime(p); !ok || born.Before(start) || entries > maxCreatedEntries {
			return errNotCreated
		}
		return nil
	})
	return err == nil
}

var errNotCreated = errors.New("not created during the session")

// within reports whether path equals or lies below an entry of set, whose
// keys are lower-case clean paths.
func within(set map[string]bool, path string) bool {
	if len(set) == 0 {
		return false
	}
	for p := strings.ToLower(filepath.Clean(path)); ; {
		if set[p] {
			return true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}

// settleNotice registers what the mode lets in from a proposal just made or
// grown. The notice survives only while candidates remain pending.
func (e *Engine) settleNotice(n Notice, err error) (*Notice, error) {
	remaining, settleErr := e.autoApprove(n.Proposal.ID)
	if remaining == nil {
		return nil, errors.Join(err, settleErr)
	}
	n.Proposal = *remaining
	return &n, errors.Join(err, settleErr)
}

// AutoApprovePending re-sorts every pending proposal under the current mode.
// The background tick runs it, so proposals made before tiering existed, before
// a mode change, or left pending by a retryable failure are settled the same
// way, and each registration it makes is announced like any other.
func (e *Engine) AutoApprovePending() error {
	if e.Svc.Proposals() == core.ProposalsAsk {
		return nil
	}
	st, err := e.Svc.Store.ReadHot()
	if err != nil {
		return err
	}
	var errs error
	for _, p := range st.Proposals {
		if p.Kind == core.ProposalWatch || p.Kind == core.ProposalWatchBurial {
			continue
		}
		if _, err := e.autoApprove(p.ID); err != nil {
			// Logged once per project and cause, like other proposal retries.
			errs = errors.Join(errs, &ProposalProjectError{Project: p.Project, Err: fmt.Errorf("auto-approve proposal #%d: %w", p.ID, err)})
		}
	}
	return errs
}

// autoApprove registers the candidates of one proposal that the mode lets in,
// through the same claim, vetting and resolution as `obit approve`, but
// without recording project consent. It returns the proposal as it remains
// pending, or nil once nothing is left.
func (e *Engine) autoApprove(id int64) (*core.Proposal, error) {
	p, ok, err := e.pendingProposal(id)
	if err != nil || !ok {
		return nil, err
	}
	allowed, class := e.autoCandidates(p)
	if len(allowed) == 0 {
		return &p, nil
	}
	claimed, release, err := e.Svc.ClaimProposalAuto(fmt.Sprintf("#%d", id))
	if errors.Is(err, core.ErrProposalBusy) {
		return &p, nil // a user decision is in progress; the next tick retries
	}
	if err != nil {
		return &p, err
	}
	defer release()
	// Only candidates unchanged since they were sorted; a refreshed revision
	// waits for the next round.
	claimed.Candidates = slices.DeleteFunc(slices.Clone(claimed.Candidates), func(c core.Candidate) bool {
		return !slices.ContainsFunc(allowed, func(a core.Candidate) bool { return a.Rev == c.Rev && sameCandidate(a, c) })
	})
	if len(claimed.Candidates) > 0 {
		e.sets = nil // reread rule files, as an approval does
		_, pending, approveErr := e.approveCandidates(claimed, locations{}, class)
		err = errors.Join(approveErr, e.Svc.ResolveProposal(claimed, pending))
	}
	rest, ok, readErr := e.pendingProposal(id)
	if !ok {
		return nil, errors.Join(err, readErr)
	}
	return &rest, errors.Join(err, readErr)
}

func (e *Engine) pendingProposal(id int64) (core.Proposal, bool, error) {
	st, err := e.Svc.Store.ReadHot()
	if err != nil {
		return core.Proposal{}, false, err
	}
	idx := st.ProposalIndex(id)
	if idx < 0 {
		return core.Proposal{}, false, nil
	}
	return st.Proposals[idx], true, nil
}

// autoCandidates sorts a pending proposal's candidates under the mode. What
// it returns still passes every approval gate; the rest stays a proposal.
func (e *Engine) autoCandidates(p core.Proposal) ([]core.Candidate, string) {
	mode := e.Svc.Proposals()
	if mode == core.ProposalsAsk {
		return nil, ""
	}
	if consent, err := e.Svc.Consent(p.Project); err != nil || consent == core.ConsentDeclined {
		return nil, "" // the user declined automatic registration in this project
	}
	switch p.Kind {
	case core.ProposalLeftovers:
		return e.regenerableCandidates(p), core.AutoRegenerable
	case core.ProposalContainer:
		// A recorded reason ties the container to the project (compose working
		// directory or bind mount); older proposals also listed containers only
		// started during the session, which may be the user's own.
		return slices.DeleteFunc(slices.Clone(p.Candidates), func(c core.Candidate) bool {
			return c.Kind != core.KindContainer || c.Reason == ""
		}), core.AutoContainer
	case core.ProposalOnboard, core.ProposalLarge:
		if mode == core.ProposalsAuto {
			return slices.Clone(p.Candidates), core.AutoAll
		}
	}
	return nil, ""
}

// regenerableCandidates keeps the leftovers Git ignores now and a tool
// recreates. Proposals from versions before 1.1.1 also listed new files Git
// does not ignore, which are uncommitted work: they stay for the user.
func (e *Engine) regenerableCandidates(p core.Proposal) []core.Candidate {
	var named []core.Candidate
	for _, c := range p.Candidates {
		if c.Kind != core.KindContainer && regenerable(c.Path) {
			named = append(named, c)
		}
	}
	if len(named) == 0 {
		return nil
	}
	ignored, err := project.Project{Root: p.Project}.Ignored()
	if err != nil {
		return nil // Git cannot say: nothing qualifies this round
	}
	set := map[string]bool{}
	for _, path := range ignored {
		set[strings.ToLower(filepath.Clean(path))] = true
	}
	return slices.DeleteFunc(named, func(c core.Candidate) bool { return !within(set, c.Path) })
}

func sameCandidate(a, b core.Candidate) bool {
	if a.Kind == core.KindContainer || b.Kind == core.KindContainer {
		return a.Kind == b.Kind && a.Container != nil && b.Container != nil && a.Container.Key() == b.Container.Key()
	}
	return strings.EqualFold(filepath.Clean(a.Path), filepath.Clean(b.Path))
}
