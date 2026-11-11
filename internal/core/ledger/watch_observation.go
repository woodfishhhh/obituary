package ledger

import (
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

const WatchStableWindow = 10 * time.Minute

type WatchSnapshot = corefs.WatchSnapshot

// WatchTiming retains the registration's age basis even after a policy edit.
type WatchTiming struct {
	Age         string        `json:"age"`
	Lifespan    string        `json:"lifespan"`
	Automatic   time.Time     `json:"automatic"`
	StableSince time.Time     `json:"stableSince"`
	Snapshot    WatchSnapshot `json:"snapshot"`
}
