package auto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/core"
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
	groups := make(map[string][]core.Proposal)
	for _, p := range st.Proposals {
		if p.Kind == core.ProposalWatch || p.Kind == core.ProposalWatchBurial || p.Kind == core.ProposalContainer {
			continue
		}
		groups[p.Project] = append(groups[p.Project], p)
	}
	var errs []error
	for _, proposals := range groups {
		root := proposals[0].Project
		if err := accessibleProposalProject(root); err != nil {
			deleted, deletionErr := e.Svc.ProposalProjectDeleted(root)
			if !deleted || deletionErr != nil {
				errs = append(errs, &ProposalProjectError{Project: root, Err: errors.Join(err, deletionErr)})
				continue
			}
			for _, p := range proposals {
				if err := e.Svc.PruneDeletedProposal(p); err != nil {
					errs = append(errs, err)
				}
			}
			continue
		}
		var paths []string
		stale := make(map[string]bool)
		for _, p := range proposals {
			for _, c := range p.Candidates {
				paths = append(paths, c.Path)
				if _, err := os.Lstat(c.Path); errors.Is(err, os.ErrNotExist) {
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
			continue
		} else {
			for i, path := range paths {
				if tracked[i] {
					stale[path] = true
				}
			}
		}
		for _, p := range proposals {
			var removed []core.Candidate
			for _, c := range p.Candidates {
				if stale[c.Path] {
					removed = append(removed, c)
				}
			}
			if err := e.Svc.PruneProposal(p, removed); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
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
func (e *Engine) Approve(refs []string) ([]core.ProposalResult, error) {
	return e.approve(refs, func(_ int, ref string) (core.Proposal, func(), error) {
		return e.Svc.ClaimProposal(ref)
	})
}

// ApproveReviewed is for delayed confirmations carrying the displayed snapshot.
func (e *Engine) ApproveReviewed(refs, fingerprints []string) ([]core.ProposalResult, error) {
	if err := core.ValidateProposalReviews(refs, fingerprints); err != nil {
		return nil, err
	}
	return e.approve(refs, func(i int, ref string) (core.Proposal, func(), error) {
		return e.Svc.ClaimProposalReviewed(ref, fingerprints[i])
	})
}

func (e *Engine) approve(refs []string, claim func(int, string) (core.Proposal, func(), error)) ([]core.ProposalResult, error) {
	e.sets = nil // reread rule files: a changed or broken .obitrules must pause registration
	var out []core.ProposalResult
	var auditErr, retryErr error
	located := locations{}
	for i, ref := range refs {
		r := core.ProposalResult{Ref: ref}
		p, release, err := claim(i, ref)
		if err != nil {
			r.Error = err.Error()
			out = append(out, r)
			continue
		}
		r.Proposal = p
		if p.Kind == core.ProposalWatchBurial {
			outcome, err := e.Svc.ApproveWatchBurial(p)
			if err == nil && outcome.Action != core.ActionBuried {
				err = errors.New(outcome.Detail)
			}
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Results = []core.Result{{Ref: outcome.Path, Item: &outcome.Item}}
			}
			release()
			out = append(out, r)
			continue
		}
		if p.Kind == core.ProposalWatch {
			err := e.Svc.ApproveWatch(p)
			if err == nil {
				err = e.Svc.ResolveProposal(p, nil)
			}
			if err != nil {
				r.Error = err.Error()
			}
			release()
			out = append(out, r)
			continue
		}
		var pending []core.Candidate
		r.Results, pending, err = e.approveCandidates(p, located)
		var audit *core.AuditError
		if errors.As(err, &audit) && core.OnlyAuditErrors(err) {
			auditErr = err
		} else if err != nil {
			retryErr = errors.Join(retryErr, err)
		}
		if err := e.Svc.ResolveProposal(p, pending); err != nil {
			r.Error = "could not save proposal outcomes; candidates remain pending: " + err.Error()
			retryErr = errors.Join(retryErr, errors.New(r.Error))
		}
		release()
		out = append(out, r)
	}
	if retryErr != nil {
		return out, retryErr
	}
	return out, auditErr
}

func (e *Engine) approveCandidates(p core.Proposal, located locations) ([]core.Result, []core.Candidate, error) {
	var out []core.Result
	var pending []core.Candidate
	var auditErr, retryErr error
	for _, c := range p.Candidates {
		results, retry, err := e.approveCandidate(p, c, located)
		out = append(out, results...)
		if retry {
			pending = append(pending, c)
		}
		var audit *core.AuditError
		if errors.As(err, &audit) {
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

func (e *Engine) approveCandidate(p core.Proposal, c core.Candidate, located locations) ([]core.Result, bool, error) {
	if c.Kind == core.KindContainer {
		return e.Svc.ApproveContainer(p, c)
	}
	res := core.Result{Ref: c.Path}
	reason, vetErr := e.vetCandidate(p, c, located)
	if err := e.checkApprovalRules(p.Project); err != nil {
		res.Error, res.Err = err.Error(), err
		return []core.Result{res}, true, err
	}
	if err := vetErr; err != nil {
		res.Error = err.Error()
		return []core.Result{res}, true, err
	} else if reason != "" {
		res.Error = reason
		return []core.Result{res}, false, nil
	}
	lifespan, err := core.ParseDuration(c.Lifespan)
	if err != nil {
		res.Error = err.Error()
		return []core.Result{res}, false, nil
	}
	note := fmt.Sprintf("approved #%d", p.ID)
	if c.Rule != "" {
		note += " (rule " + c.Rule + ")"
	}
	results, err := e.Svc.Add([]string{c.Path}, core.AddOptions{
		ExpiresAt: e.now().Add(lifespan), Source: "proposal", Project: p.Project, Session: core.NormalizeSession(p.Session), Note: note, OnlyNew: true,
		BeforeCommit: func() error { return e.checkApprovalRules(p.Project) },
	})
	var audit *core.AuditError
	if err != nil && !errors.As(err, &audit) {
		res.Error = err.Error()
		return []core.Result{res}, true, err
	}
	retry := false
	if core.Failed(results) {
		// Only definitive inspection failures settle a candidate.
		var refused *core.RefusedError
		cause := results[0].Err
		retry = !errors.As(cause, &refused) && !errors.Is(cause, core.ErrUnsafe) && !errors.Is(cause, core.ErrNotFound)
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

func (e *Engine) vetCandidate(p core.Proposal, c core.Candidate, located locations) (string, error) {
	info, err := os.Lstat(c.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "no longer exists", nil
		}
		return "", err
	}
	real, err := e.Svc.FS.RealPath(c.Path)
	if err != nil {
		if errors.Is(err, core.ErrUnsafe) {
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
		return core.ErrProtected.Error(), nil
	}
	state, err := e.Svc.Store.ReadHot()
	if err != nil {
		return "", err
	}
	return e.vetUnit(proj, real, &state, located)
}
