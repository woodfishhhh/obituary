package ledger

import (
	"time"
)

// BurialOperation identifies one explicit invocation, including callers that
// validate and bury one operand at a time (rm). Item.Session/Project describe
// registration, so they cannot establish who performed a later deletion.
type BurialOperation struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"startedAt"`
	Session   string    `json:"session,omitempty"`
	Project   string    `json:"project,omitempty"`
}
