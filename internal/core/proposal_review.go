package core

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
)

var ErrProposalChanged = errors.New("提议已变化，请重新查看")

// WatchReview is the consent-sensitive policy saved with its proposal.
// Missing snapshots from older ledgers require a fresh SetWatch review.
type WatchReview struct {
	ID       int64    `json:"id"`
	Rev      int64    `json:"rev"`
	Path     string   `json:"path"`
	Project  string   `json:"project"`
	Identity Identity `json:"identity"`
	Lifespan string   `json:"lifespan"`
	Age      string   `json:"age"`
}

func watchReview(p WatchPolicy) WatchReview {
	return WatchReview{p.ID, p.Rev, p.Path, p.Project, p.Identity, p.Lifespan, p.Age}
}

func watchProposal(p WatchPolicy, id int64) Proposal {
	rule := "folder policy, age=" + p.Age
	if p.Age == WatchModified {
		rule += "; modified dates may expire immediately"
	}
	review := watchReview(p)
	return Proposal{ID: id, Kind: ProposalWatch, WatchID: p.ID, Project: p.Path,
		WatchReview: &review,
		Candidates:  []Candidate{{Path: p.Path, Lifespan: p.Lifespan, Rev: uint64(p.Rev), Rule: rule}}}
}

func hasWatchReview(st *State, policy WatchPolicy) bool {
	return policy.Consent != "pending" || slices.ContainsFunc(st.Proposals, func(p Proposal) bool {
		return p.WatchID == policy.ID && checkWatchReview(p, policy) == nil
	})
}

func checkWatchReview(p Proposal, current WatchPolicy) error {
	if p.WatchReview == nil || *p.WatchReview != watchReview(current) {
		return ErrProposalChanged
	}
	expected := watchProposal(current, p.ID)
	if p.Kind != expected.Kind || p.WatchID != expected.WatchID || p.Project != expected.Project ||
		!slices.Equal(p.Candidates, expected.Candidates) {
		return ErrProposalChanged
	}
	return nil
}

// ProposalFingerprint binds a decision to the complete reviewed snapshot.
// Notification bookkeeping is excluded; candidate revisions never get reused.
func ProposalFingerprint(p Proposal) string {
	p.Notified = nil
	p.Candidates = slices.Clone(p.Candidates)
	slices.SortFunc(p.Candidates, func(a, b Candidate) int {
		return cmp.Compare(a.Path, b.Path)
	})
	data, _ := json.Marshal(struct {
		ID, WatchID            int64
		Kind, Project, Session string
		CandidateSeq           uint64
		Candidates             []Candidate
		WatchReview            *WatchReview
		WatchBurial            *WatchBurialReview
	}{p.ID, p.WatchID, p.Kind, p.Project, p.Session, p.CandidateSeq, p.Candidates, p.WatchReview, p.WatchBurial})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ClaimProposalReviewed compares within the same ledger transaction that
// grants project consent and claims the candidates.
func (s *Service) ClaimProposalReviewed(ref, fingerprint string) (Proposal, func(), error) {
	return s.claimProposal(ref, func(p Proposal) error {
		return checkProposalReview(p, fingerprint)
	})
}

func checkProposalReview(p Proposal, fingerprint string) error {
	if fingerprint == "" || ProposalFingerprint(p) != fingerprint {
		return ErrProposalChanged
	}
	return nil
}

func ValidateProposalReviews(refs, fingerprints []string) error {
	if len(refs) != len(fingerprints) {
		return ErrProposalChanged
	}
	for _, fingerprint := range fingerprints {
		if fingerprint == "" {
			return ErrProposalChanged
		}
	}
	return nil
}

// RejectReviewed preserves every proposal whose snapshot has changed.
func (s *Service) RejectReviewed(refs, fingerprints []string) ([]ProposalResult, error) {
	if err := ValidateProposalReviews(refs, fingerprints); err != nil {
		return nil, err
	}
	return s.reject(refs, func(i int, p Proposal) error {
		return checkProposalReview(p, fingerprints[i])
	})
}
