package ledger

import "slices"

// Graveyard quota proposal kind, audit action, and FIFO purge basis.
const (
	ProposalGraveyardQuota = "graveyard-quota"
	ActionQuotaExceeded    = "quota-exceeded"
	PurgeBasisQuotaFIFO    = "graveyard_capacity_fifo"
)

// QuotaBurialReview binds a pending graveyard-quota decision to the exact
// oversized object that was left safely in place instead of moving into the graveyard.
type QuotaBurialReview struct {
	ItemID    int64    `json:"itemId,omitempty"`
	ItemRev   int64    `json:"itemRev,omitempty"`
	Identity  Identity `json:"identity"`
	ItemBytes int64    `json:"itemBytes"`
	MaxBytes  int64    `json:"maxBytes"`
	Reason    string   `json:"reason,omitempty"`
	IsDir     bool     `json:"isDir,omitempty"`
}

// AwaitingQuotaDecision reports whether an expired item is already waiting in
// pending decisions because its size exceeded maxBytes. Raising or clearing
// maxBytes automatically unblocks the item for the next sweep.
func AwaitingQuotaDecision(st *State, it Item, maxBytes int64) bool {
	if maxBytes <= 0 || it.Identity == nil {
		return false
	}
	return slices.ContainsFunc(st.Proposals, func(p Proposal) bool {
		if p.Kind != ProposalGraveyardQuota || p.QuotaReview == nil {
			return false
		}
		r := p.QuotaReview
		if r.ItemBytes <= maxBytes {
			return false
		}
		if r.Identity == *it.Identity && (r.ItemID == 0 || r.ItemID == it.ID && r.ItemRev == it.Rev) {
			return true
		}
		return false
	})
}

// QuotaPendingCount returns how many pending graveyard-quota decisions exist.
func QuotaPendingCount(st State) int {
	n := 0
	for _, p := range st.Proposals {
		if p.Kind == ProposalGraveyardQuota {
			n++
		}
	}
	return n
}
