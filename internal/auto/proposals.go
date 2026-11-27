package auto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/guard"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	corelifespan "github.com/woodfishhhh/obituary/internal/core/lifespan"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/rules"
)

// RefreshProposals runs only in the background/reap path. Batch each project's
// candidates into bounded Git queries to limit subprocess overhead. Query each
// round afresh so changed tracking state is reconsidered.
func (e *Engine) RefreshProposals() error {
	st, err := e.Svc.Store.ReadHot()
	if err != nil {
		return err
	}
	groups := make(map[string][]ledger.Proposal)
	for _, p := range st.Proposals {
		if p.Kind == ledger.ProposalWatch || p.Kind == ledger.ProposalWatchBurial || p.Kind == ledger.ProposalContainer || p.Kind == ledger.ProposalGraveyardQuota {
			continue
		}
		groups[p.Project] = append(groups[p.Project], p)
	}
	var errs []error
	for _, proposals := range groups {
		errs = append(errs, e.refreshProjectProposals(proposals)...)
	}
	// Sort what is still pending under the current proposal mode: proposals
	// made before tiering, before a mode change, or left by a retryable failure.
	return errors.Join(append(errs, e.AutoApprovePending())...)
}

func (e *Engine) refreshProjectProposals(proposals []ledger.Proposal) []error {
	root := proposals[0].Project
	if err := accessibleProposalProject(root); err != nil {
		return e.pruneDeletedProposals(proposals, err)
	}
	var errs []error
	var paths []string
	stale := make(map[string]bool)
	for _, p := range proposals {
		for _, c := range p.Candidates {
			paths = append(paths, c.Path)
			if c.IsFile() && NeverAutomatic(root, c.Path) != "" {
				stale[c.Path] = true // proposed before dependencies and build output were left alone
			} else if _, err := os.Lstat(c.Path); errors.Is(err, os.ErrNotExist) {
				stale[c.Path] = true
			} else if err != nil {
				errs = append(errs, err)
			}
		}
	}
	tracked, err := (project.Project{Root: root}).TrackedBatch(paths)
	if err != nil {
		// Git failures prevent registration, but are not evidence for
		// withdrawing a proposal, including missing candidates in worktrees
		// whose .git pointer survives unavailable main-repository metadata.
		errs = append(errs, &ProposalProjectError{Project: root, Err: err})
		return errs
	} else {
		for i, path := range paths {
			if tracked[i] {
				stale[path] = true
			}
		}
	}
	for _, p := range proposals {
		var removed []ledger.Candidate
		for _, c := range p.Candidates {
			if stale[c.Path] {
				removed = append(removed, c)
			}
		}
		if err := e.Svc.PruneProposal(p, removed); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (e *Engine) pruneDeletedProposals(proposals []ledger.Proposal, cause error) []error {
	root := proposals[0].Project
	deleted, err := e.Svc.ProposalProjectDeleted(root)
	if !deleted || err != nil {
		return []error{&ProposalProjectError{Project: root, Err: errors.Join(cause, err)}}
	}
	var errs []error
	for _, p := range proposals {
		if err := e.Svc.PruneDeletedProposal(p); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// ProposalProjectError preserves the cause while allowing runners to log state
// transitions independently for each project.
type ProposalProjectError struct {
	Project string
	Err     error
}

func (e *ProposalProjectError) Error() string { return e.Err.Error() }
func (e *ProposalProjectError) Unwrap() error { return e.Err }

// An unavailable worktree needs independent evidence of deletion.
// Check .git too: a detached mount may leave an accessible empty directory.
func accessibleProposalProject(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("proposal project %q is unavailable; retry later: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("proposal project %q is not a directory; retry later", root)
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); err != nil {
		return fmt.Errorf("proposal project %q has unavailable Git metadata; retry later: %w", root, err)
	}
	return nil
}

// Approve claims proposals and registers each candidate after vetting it
// again on its real location: still in the proposal's project, unprotected,
// untracked and not shortening a lifespan. Anything that changed since the
// proposal was made is skipped, not registered.
func (e *Engine) Approve(refs []string) ([]lifecycle.ProposalResult, error) {
	return e.approve(refs, func(_ int, ref string) (ledger.Proposal, func(), error) {
		return e.Svc.ClaimProposal(ref)
	})
}

// ApproveReviewed is for delayed confirmations carrying the displayed snapshot.
func (e *Engine) ApproveReviewed(refs, fingerprints []string) ([]lifecycle.ProposalResult, error) {
	if err := lifecycle.ValidateProposalReviews(refs, fingerprints); err != nil {
		return nil, err
	}
	return e.approve(refs, func(i int, ref string) (ledger.Proposal, func(), error) {
		return e.Svc.ClaimProposalReviewed(ref, fingerprints[i])
	})
}

func (e *Engine) approve(refs []string, claim func(int, string) (ledger.Proposal, func(), error)) ([]lifecycle.ProposalResult, error) {
	e.sets = nil // reread rule files: a changed or broken .obitrules must pause registration
	var out []lifecycle.ProposalResult
	var auditErr, retryErr error
	located := locations{}
	for i, ref := range refs {
		r := lifecycle.ProposalResult{Ref: ref}
		p, release, err := claim(i, ref)
		if err != nil {
			r.Error = err.Error()
			if errors.Is(err, lifecycle.ErrProposalChanged) {
				r.ErrorCode = lifecycle.ErrorCodeProposalChanged
			}
			out = append(out, r)
			continue
		}
		r.Proposal = p
		if e.approveWatchProposal(p, &r) {
			release()
			out = append(out, r)
			continue
		}
		var pending []ledger.Candidate
		r.Results, pending, err = e.approveCandidates(p, located, "")
		if ledger.HasAuditWarning(err) && ledger.OnlyAuditErrors(err) {
			auditErr = err
		} else if err != nil {
			retryErr = errors.Join(retryErr, err)
		}
		if err := e.Svc.ResolveProposal(p, pending); err != nil {
			r.Error = "could not save proposal outcomes; candidates remain pending: " + err.Error()
			retryErr = errors.Join(retryErr, errors.New(r.Error))
		} else {
			for _, c := range pending {
				r.Remaining = append(r.Remaining, c.Path)
			}
		}
		release()
		out = append(out, r)
	}
	if retryErr != nil {
		return out, retryErr
	}
	return out, auditErr
}

func (e *Engine) approveWatchProposal(p ledger.Proposal, r *lifecycle.ProposalResult) bool {
	switch p.Kind {
	case ledger.ProposalWatchBurial:
		e.approveWatchBurialProposal(p, r)
	case ledger.ProposalWatch:
		e.approveWatchPolicyProposal(p, r)
	case ledger.ProposalGraveyardQuota:
		e.approveQuotaProposal(p, r)
	default:
		return false
	}
	return true
}

func (e *Engine) approveWatchBurialProposal(p ledger.Proposal, r *lifecycle.ProposalResult) {
	outcome, err := e.Svc.ApproveWatchBurial(p)
	if err == nil && outcome.Action != ledger.ActionBuried {
		err = errors.New(outcome.Detail)
	}
	if err != nil {
		r.Error = err.Error()
		if errors.Is(err, lifecycle.ErrProposalChanged) {
			r.ErrorCode = lifecycle.ErrorCodeProposalChanged
		}
		return
	}
	r.Results = []lifecycle.Result{{Ref: outcome.Path, Item: &outcome.Item}}
	r.QuotaEviction = outcome.QuotaEviction
}

func (e *Engine) approveWatchPolicyProposal(p ledger.Proposal, r *lifecycle.ProposalResult) {
	err := e.Svc.ApproveWatch(p)
	if err == nil {
		err = e.Svc.ResolveProposal(p, nil)
	}
	if err != nil {
		r.Error = err.Error()
		if errors.Is(err, lifecycle.ErrProposalChanged) {
			r.ErrorCode = lifecycle.ErrorCodeProposalChanged
		}
	}
}

func (e *Engine) approveQuotaProposal(p ledger.Proposal, r *lifecycle.ProposalResult) {
	outcome, err := e.Svc.ApproveQuotaProposal(p)
	if err == nil && outcome.Action != ledger.ActionBuried && outcome.Action != ledger.ActionRecycled {
		err = errors.New(outcome.Detail)
	}
	if err != nil {
		r.Error = err.Error()
		if errors.Is(err, lifecycle.ErrProposalChanged) {
			r.ErrorCode = lifecycle.ErrorCodeProposalChanged
		}
		return
	}
	r.Results = []lifecycle.Result{{Ref: outcome.Path, Item: &outcome.Item}}
	r.QuotaEviction = outcome.QuotaEviction
}

func (e *Engine) approveCandidates(p ledger.Proposal, located locations, class string) ([]lifecycle.Result, []ledger.Candidate, error) {
	var out []lifecycle.Result
	var pending []ledger.Candidate
	var auditErr, retryErr error
	for _, c := range p.Candidates {
		results, retry, err := e.approveCandidate(p, c, located, class)
		out = append(out, results...)
		if retry {
			pending = append(pending, c)
		}
		if ledger.HasAuditWarning(err) {
			auditErr = err
		} else if err != nil {
			retryErr = errors.Join(retryErr, err)
		}
	}
	if retryErr != nil {
		return out, pending, retryErr
	}
	return out, pending, auditErr
}

func (e *Engine) approveCandidate(p ledger.Proposal, c ledger.Candidate, located locations, class string) ([]lifecycle.Result, bool, error) {
	if c.Kind == ledger.KindContainer {
		return e.approveContainer(p, c, class)
	}
	res := lifecycle.Result{Ref: c.Path}
	reason, vetErr := e.vetCandidate(p, c, located)
	if err := e.checkApprovalRules(p.Project); err != nil {
		res.Error, res.Err = err.Error(), err
		return []lifecycle.Result{res}, true, err
	}
	if err := vetErr; err != nil {
		res.Error = err.Error()
		return []lifecycle.Result{res}, true, err
	} else if reason != "" {
		res.Error = reason
		return []lifecycle.Result{res}, false, nil
	}
	lifespan, err := corelifespan.ParseDuration(c.Lifespan)
	if err != nil {
		res.Error = err.Error()
		return []lifecycle.Result{res}, false, nil
	}
	note := fmt.Sprintf("approved #%d", p.ID)
	if class != "" {
		note = fmt.Sprintf("auto-approved #%d (%s)", p.ID, class)
	}
	if c.Rule != "" {
		note += " (rule " + c.Rule + ")"
	}
	results, err := e.Svc.Add([]string{c.Path}, lifecycle.AddOptions{
		ExpiresAt: e.now().Add(lifespan), Source: "proposal", Project: p.Project, Session: lifecycle.NormalizeSession(p.Session), Note: note, OnlyNew: true,
		AutoClass:    class,
		BeforeCommit: func() error { return e.checkApprovalRules(p.Project) },
	})
	if err != nil && !ledger.HasAuditWarning(err) {
		res.Error = err.Error()
		return []lifecycle.Result{res}, true, err
	}
	retry := false
	if lifecycle.Failed(results) {
		// Only definitive inspection failures settle a candidate.
		var refused *guard.RefusedError
		cause := results[0].Err
		retry = !errors.As(cause, &refused) && !errors.Is(cause, corefs.ErrUnsafe) && !errors.Is(cause, corefs.ErrNotFound)
	}
	return results, retry, err
}

// Keep the batch's first rule snapshot as its baseline. Changes leave every
// affected candidate pending, including a candidate previously protected.
func (e *Engine) checkApprovalRules(root string) error {
	_, reviewed, err := e.RuleSet(root)
	if err != nil {
		return fmt.Errorf("rules: %w", err)
	}
	_, current, err := rules.Load(root, e.Trust)
	if err != nil {
		return fmt.Errorf("rules: %w", err)
	}
	if current != reviewed {
		return errors.New("rules or trust changed during approval; review and retry")
	}
	return nil
}

func (e *Engine) vetCandidate(p ledger.Proposal, c ledger.Candidate, located locations) (string, error) {
	info, err := os.Lstat(c.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "no longer exists", nil
		}
		return "", err
	}
	real, err := e.Svc.FS.RealPath(c.Path)
	if err != nil {
		if errors.Is(err, corefs.ErrUnsafe) {
			return err.Error(), nil
		}
		return "", fmt.Errorf("could not resolve its real location: %w", err)
	}
	proj, ok, err := project.Find(real)
	if err != nil {
		return "", fmt.Errorf("could not find its Git work tree: %w", err)
	}
	if !ok || !strings.EqualFold(proj.Root, p.Project) {
		return "it is no longer inside " + p.Project, nil
	}
	set, _, err := e.RuleSet(proj.Root)
	if err != nil {
		return "", fmt.Errorf("rules: %w", err)
	}
	protected, err := set.Protects(real, info.IsDir())
	if err != nil {
		return "", fmt.Errorf("could not check protection rules: %w", err)
	}
	if protected {
		return lifecycle.ErrProtected.Error(), nil
	}
	state, err := e.Svc.Store.ReadHot()
	if err != nil {
		return "", err
	}
	return e.vetUnit(proj, real, &state, located)
}
