package ledger

import (
	"time"
)

// AutoExemption reuses watch's object identity and handle-bound metadata.
// Unlike a watch keep decision, an automatic-registration exemption ends on
// a later write. KeptAt prevents delayed pre-restore hooks from undoing it.
type AutoExemption struct {
	Identity Identity       `json:"identity"`
	KeptAt   time.Time      `json:"keptAt"`
	Snapshot *WatchSnapshot `json:"snapshot,omitempty"`
}
