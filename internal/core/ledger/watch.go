package ledger

import (
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
