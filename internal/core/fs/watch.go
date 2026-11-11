package fs

import (
	"time"
)

// WatchSnapshot binds metadata to the same open object as its identity.
type WatchSnapshot struct {
	Identity  Identity  `json:"identity"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	Tree      string    `json:"tree,omitempty"`
	Directory bool      `json:"directory,omitempty"`
}
