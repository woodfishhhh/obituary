package ledger

import (
	"time"
)

func (st *State) ObservationCompleted(row *Observation) bool {
	if row == nil {
		return false
	}
	_, done := st.ObservationReceipts[row.Token]
	return done
}

// RememberObservation is committed with Add or ProposeObserved, never after
// their transaction. Receipts outlive queue acknowledgement and user decisions.
// Expired requests are never replayed, so receipts older than their seven-day
// admission window can be pruned without permitting an old write again.
func (st *State) RememberObservation(row *Observation, now time.Time) {
	if row == nil {
		return
	}
	for token, at := range st.ObservationReceipts {
		if now.Sub(at) >= ObservationMaxAge {
			delete(st.ObservationReceipts, token)
		}
	}
	if st.ObservationReceipts == nil {
		st.ObservationReceipts = make(map[string]time.Time)
	}
	st.ObservationReceipts[row.Token] = row.ObservedAt
}

// CompleteObservation records terminal non-registration decisions before the
// queue file is acknowledged. Audit first: a failed audit keeps it retryable.
func (s Store) CompleteObservation(row Observation, now time.Time, reason string) error {
	if reason != "" {
		if err := s.Record(Event{Time: now, Action: "observation skipped", Path: row.Path, Detail: reason}); err != nil {
			return err
		}
	}
	return s.UpdateHot(func(st *State) error { st.RememberObservation(&row, now); return nil })
}
