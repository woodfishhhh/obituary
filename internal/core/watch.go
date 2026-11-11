package core

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	WatchFirstSeen = "first-seen"
	WatchModified  = "modified"
	ProposalWatch  = "watch-policy"
)

// WatchPolicy manages only the immediate children of this pinned directory.
// Changing a consent-sensitive policy requires a new decision; binding approval
// to the saved revision prevents policy edits from inheriting stale consent.
type WatchPolicy struct {
	ID          int64            `json:"id"`
	Rev         int64            `json:"rev"`
	Path        string           `json:"path"`
	Project     string           `json:"project,omitempty"`
	Identity    Identity         `json:"identity"`
	Lifespan    string           `json:"lifespan"`
	Age         string           `json:"age"`
	Consent     string           `json:"consent"`
	ApprovedRev int64            `json:"approvedRev,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	Aliases     []string         `json:"aliases,omitempty"`
	Exclusions  []WatchExclusion `json:"exclusions,omitempty"`
}

// Watches returns persisted policies, including policies waiting for approval.
func (s *Service) Watches() ([]WatchPolicy, error) {
	st, err := s.Store.Read()
	return st.Watches, err
}

// SetWatch is an explicit request to manage a directory's children. Existing
// lifespans are preserved; a changed duration applies to new registrations.
func (s *Service) SetWatch(path, lifespan, age string) (WatchPolicy, error) {
	var out WatchPolicy
	if _, err := ParseDuration(lifespan); err != nil {
		return out, err
	}
	if age != WatchFirstSeen && age != WatchModified {
		return out, fmt.Errorf("age must be first-seen or modified")
	}
	alias, _, err := s.FS.LongPath(path)
	if err != nil {
		return out, err
	}
	path, identity, err := s.watchDirectory(path)
	if err != nil {
		return out, err
	}
	root := ""
	if s.Owner != nil {
		// Owner's Git pathspec contract requires a strict child of its root.
		// This read-only metadata lookup also works for the worktree root.
		root, _, err = s.Owner(filepath.Join(path, ".git"))
		if err != nil {
			return out, err
		}
	}
	return s.saveWatch(WatchPolicy{Path: path, Identity: identity, Project: root, Lifespan: lifespan, Age: age, Aliases: []string{alias}})
}

func (s *Service) saveWatch(policy WatchPolicy) (WatchPolicy, error) {
	out := policy
	err := s.Store.Update(func(st *State) error {
		out.ID, out.Rev, out.Consent, out.CreatedAt = st.NextID, 1, "pending", s.now()
		idx := slices.IndexFunc(st.Watches, func(p WatchPolicy) bool { return sameWatchIdentity(p.Identity, out.Identity) || p.Path == out.Path })
		if idx >= 0 {
			old := st.Watches[idx]
			out.Exclusions = old.Exclusions
			out.Aliases = slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(old.Aliases), out.Aliases...))))
			oldDuration, _ := ParseDuration(old.Lifespan)
			newDuration, _ := ParseDuration(out.Lifespan)
			reviewed := out
			reviewed.ID, reviewed.Rev, reviewed.Consent = old.ID, old.Rev, old.Consent
			if sameWatchIdentity(old.Identity, out.Identity) && old.Project == out.Project && oldDuration == newDuration && old.Age == out.Age &&
				(old.Consent != ConsentApproved || old.explicitlyApproved()) && hasWatchReview(st, reviewed) {
				out.ID, out.Rev, out.Consent, out.CreatedAt = old.ID, old.Rev, old.Consent, old.CreatedAt
				out.ApprovedRev = old.ApprovedRev
				st.Watches[idx] = out
				return nil
			}
			out.ID, out.Rev, out.CreatedAt = old.ID, old.Rev+1, old.CreatedAt
			st.Proposals = slices.DeleteFunc(st.Proposals, func(p Proposal) bool { return p.WatchID == old.ID })
		} else {
			st.NextID++
			idx = len(st.Watches)
			st.Watches = append(st.Watches, out)
		}
		if out.Consent == "pending" {
			st.proposeWatch(out, s.now())
		}
		st.Watches[idx] = out
		return nil
	})
	return out, err
}

// ApproveWatch is called while the existing proposal claim is held.
func (s *Service) ApproveWatch(proposal Proposal) error {
	return s.Store.Update(func(st *State) error {
		idx := slices.IndexFunc(st.Watches, func(p WatchPolicy) bool { return p.ID == proposal.WatchID })
		if idx < 0 || len(proposal.Candidates) != 1 || st.ProposalIndex(proposal.ID) < 0 {
			return fmt.Errorf("folder policy proposal is no longer pending")
		}
		p := &st.Watches[idx]
		if err := checkWatchReview(proposal, *p); err != nil {
			return err
		}
		if err := checkProposalReview(st.Proposals[st.ProposalIndex(proposal.ID)], ProposalFingerprint(proposal)); err != nil {
			return err
		}
		_, id, err := s.watchDirectory(p.Path)
		if err != nil {
			return err
		}
		if id != p.Identity {
			return fmt.Errorf("watched directory was replaced: %w", ErrUnsafe)
		}
		p.Consent = ConsentApproved
		p.ApprovedRev = p.Rev
		return nil
	})
}

func (s *Service) watchDirectory(path string) (string, Identity, error) {
	path, _, err := s.FS.LongPath(path)
	if err != nil {
		return "", Identity{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", Identity{}, err
	}
	if !info.IsDir() || isLink(info) {
		return "", Identity{}, fmt.Errorf("watch requires an ordinary directory: %w", ErrUnsafe)
	}
	path, err = s.FS.RealPath(path)
	if err != nil {
		return "", Identity{}, err
	}
	if insideGitMetadata(path) {
		return "", Identity{}, &RefusedError{Path: path, Reason: "Git metadata cannot have a folder policy"}
	}
	if reason := s.watchGuard().protected(path); reason != "" {
		return "", Identity{}, &RefusedError{Path: path, Reason: reason}
	}
	id, err := s.FS.Identify(path)
	return filepath.Clean(path), id, err
}

func insideGitMetadata(path string) bool {
	for parent := filepath.Dir(path); parent != path; parent = filepath.Dir(path) {
		if strings.EqualFold(filepath.Base(path), ".git") {
			return true
		}
		path = parent
	}
	return false
}
