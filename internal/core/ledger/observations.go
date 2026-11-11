package ledger

import (
	"time"
)

const (
	observationBytes       = 1 << 20
	ObservationMaxAge      = 7 * 24 * time.Hour
	ObservationMaxAttempts = 20
)

// Observation is a deferred hook write. Token prevents acknowledgement of an
// older replay from deleting a newer write to the same path in that session.
type Observation struct {
	Path       string    `json:"path"`
	Host       string    `json:"host,omitempty"`
	Event      string    `json:"event,omitempty"`
	Project    string    `json:"project,omitempty"`
	Session    string    `json:"session"`
	Token      string    `json:"token"`
	Identity   *Identity `json:"identity,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
	Attempts   int       `json:"attempts,omitempty"`
	LastError  string    `json:"lastError,omitempty"`
}
