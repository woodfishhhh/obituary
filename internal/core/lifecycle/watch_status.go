package lifecycle

import (
	"time"
)

// WatchStatus describes the saved observation; a new write starts a fresh
// stable window when the watcher next observes it. Directories await review.
type WatchStatus struct {
	StableWindow string    `json:"stableWindow"`
	QuietFor     string    `json:"quietFor"`
	StableAt     time.Time `json:"stableAt"`
	NextCheck    time.Time `json:"nextCheck"`
}

func (s *Service) WatchStatus(it Item, now time.Time) *WatchStatus {
	if it.Source != "watch" || it.IsDir || it.Watch == nil || it.Watch.StableSince.IsZero() {
		return nil
	}
	window := s.watchStableWindow()
	start := it.Watch.StableSince
	if posixWatchFile(it) {
		window = max(WatchStableWindow, window)
		if start.Before(it.ExpiresAt) {
			start = it.ExpiresAt
		}
	}
	stableAt := start.Add(window)
	next := it.ExpiresAt
	if it.NextAttempt.After(next) {
		next = it.NextAttempt
	}
	if next.Before(now) {
		next = now
	}
	remaining := max(0, stableAt.Sub(now))
	// Round up so a sub-second wait is never displayed as already quiet.
	remaining = ((remaining + time.Second - 1) / time.Second) * time.Second
	return &WatchStatus{StableWindow: window.String(), QuietFor: remaining.String(), StableAt: stableAt, NextCheck: next}
}
