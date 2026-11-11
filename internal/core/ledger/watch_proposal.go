package ledger

import (
	"slices"
)

const ProposalWatchBurial = "watch-burial"

const ActionProposed = "proposed"

// WatchBurialReview binds the decision to a lifespan and object, not a name.
// Counts and kept paths describe the displayed tree; final movement rechecks it.
type WatchBurialReview struct {
	ItemID   int64    `json:"itemId"`
	ItemRev  int64    `json:"itemRev"`
	Identity Identity `json:"identity"`
	Files    int64    `json:"files"`
	Kept     []string `json:"kept,omitempty"`
}

func AwaitingWatchBurial(st *State, it Item) bool {
	if it.Source != "watch" || !it.IsDir {
		return false
	}
	return slices.ContainsFunc(st.Proposals, func(p Proposal) bool {
		return p.Kind == ProposalWatchBurial && p.WatchBurial != nil && p.WatchBurial.ItemID == it.ID &&
			p.WatchID == it.WatchID && p.WatchBurial.ItemRev == it.Rev && it.Identity != nil && p.WatchBurial.Identity == *it.Identity
	})
}
