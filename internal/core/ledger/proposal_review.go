package ledger

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

func WatchReviewOf(p WatchPolicy) WatchReview {
	return WatchReview{p.ID, p.Rev, p.Path, p.Project, p.Identity, p.Lifespan, p.Age}
}

func WatchProposal(p WatchPolicy, id int64) Proposal {
	rule := "folder policy, age=" + p.Age
	if p.Age == WatchModified {
		rule += "; modified dates may expire immediately"
	}
	review := WatchReviewOf(p)
	return Proposal{ID: id, Kind: ProposalWatch, WatchID: p.ID, Project: p.Path,
		WatchReview: &review,
		Candidates:  []Candidate{{Path: p.Path, Lifespan: p.Lifespan, Rev: uint64(p.Rev), Rule: rule}}}
}
