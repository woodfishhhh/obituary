package ledger

import (
	"time"
)

// WatchExemption records an explicit user keep decision for one policy/object.
type WatchExemption struct {
	WatchID  int64     `json:"watchId"`
	Identity Identity  `json:"identity"`
	Path     string    `json:"path,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	KeptAt   time.Time `json:"keptAt,omitempty"`
}
