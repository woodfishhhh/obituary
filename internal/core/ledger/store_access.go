package ledger

import (
	"os"
	"time"
)

// WithLock shares the operation's context and deadline with its outer locks.
func (s Store) WithLock(name string, exclusive bool, fn func() error) error {
	return s.withLock(name, exclusive, fn)
}

// LockDeadline is the absolute deadline shared by every lock in an operation.
func (s Store) LockDeadline() time.Time { return s.lockDeadline }

// BeginAuditBatch keeps nested operations in the outer batch's single flush.
// The returned flag tells the caller whether it owns a new request copy.
func (s Store) BeginAuditBatch() (Store, func() error, bool) {
	if s.auditPending != nil {
		return s, func() error { return nil }, false
	}
	s.auditPending = new(bool)
	return s, func() error {
		if !*s.auditPending {
			return nil
		}
		store := s
		store.auditPending = nil
		return store.FlushAudit()
	}, true
}

// WithAuditCounter preserves the request's shared recovery-advice counter.
func (s Store) WithAuditCounter(counter *int) Store { s.auditLines = counter; return s }

// WithChangeFlag records visible commits for callbacks after locks are released.
func (s Store) WithChangeFlag(changed *bool) Store { s.changed = changed; return s }

// WithLockObserver measures this store's lock ownership without changing its scope.
func (s Store) WithLockObserver(observe func(time.Duration)) Store { s.observeLock = observe; return s }

// LoadGraveyard joins the snapshot while the caller holds the hot transaction lock.
func (s Store) LoadGraveyard(st *State) error { return s.loadGraveyard(st) }

// LoadGraveyardForReference uses the committed safety index under the same lock.
func (s Store) LoadGraveyardForReference(st *State, ref string) error {
	return s.loadGraveyardForReference(st, ref)
}

// GuardItems refreshes a caller's active index on atomic ledger replacement.
// The shared lock covers the final filesystem operation, never a directory walk.
func (s Store) GuardItems(refresh func([]Item) error) func(func() error) error {
	var previous os.FileInfo
	return func(use func() error) error {
		return s.withLock("store.lock", false, func() error {
			f, err := os.Open(s.statePath())
			if err != nil {
				return err
			}
			stamp, err := f.Stat()
			_ = f.Close()
			if err != nil {
				return err
			}
			if previous == nil || !os.SameFile(previous, stamp) || previous.Size() != stamp.Size() || previous.ModTime() != stamp.ModTime() {
				st, _, err := s.loadHot()
				if err != nil {
					return err
				}
				if err := refresh(st.Items); err != nil {
					return err
				}
				previous = stamp
			}
			return use()
		})
	}
}
