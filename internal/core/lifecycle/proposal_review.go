package lifecycle

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

var ErrProposalChanged = errors.New("提议已变化，请重新查看")

type WatchReview = ledger.WatchReview

func watchReview(p WatchPolicy) WatchReview { return ledger.WatchReviewOf(p) }

func watchProposal(p WatchPolicy, id int64) Proposal { return ledger.WatchProposal(p, id) }

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
	}, true)
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
