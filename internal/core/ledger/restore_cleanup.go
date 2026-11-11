package ledger

import (
	"time"
)

// RestoreCleanup is committed with removal of the burial index and retains
// the location and identities needed to finish either restore outcome.
type RestoreCleanup struct {
	Kind        string         `json:"kind"`
	Burial      Burial         `json:"burial"`
	Path        string         `json:"path,omitempty"`
	Restore     *RestoreIntent `json:"resurrect,omitempty"`
	CommittedAt time.Time      `json:"committedAt"`
}
