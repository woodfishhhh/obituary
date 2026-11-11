package ledger

import (
	"context"
	"errors"
	"fmt"
)

func PendingAuditCount(st *State) int {
	count := len(st.PendingAudit)
	for _, p := range st.ContainerIntents {
		count++
		if p.Diagnostic == "" {
			count++
		}
	}
	if !st.graveyardLoaded && st.GraveyardIndex != nil {
		return count + st.GraveyardIndex.ReservedAudit
	}
	for _, b := range st.Burials {
		if b.PurgeIntent != nil && FindContainerIntent(st, b.ID) == nil {
			count++
		}
	}
	return count
}

// FlushAudit only delivers committed events; it never repeats an operation or
// changes statistics. A crash after append and before removing the queue can
// repeat an event (at least once). Only this function removes the FIFO prefix.
func (s Store) FlushAudit() error {
	s = s.WithContext(context.WithoutCancel(s.Context()))
	err := s.withLock("audit-outbox.lock", true, func() error {
		st, err := s.ReadHot()
		if err != nil || len(st.PendingAudit) == 0 {
			return err
		}
		if s.auditLines != nil {
			// Include the whole attempted append: a failed append may already
			// have written a prefix, and the log advice must cover it too.
			*s.auditLines += len(st.PendingAudit)
		}
		if err := s.RecordCommitted(st.PendingAudit...); err != nil {
			return err
		}
		return s.UpdateHot(func(current *State) error {
			current.PendingAudit = current.PendingAudit[len(st.PendingAudit):]
			return nil
		})
	})
	if err != nil {
		return &AuditError{Err: fmt.Errorf("pending audit delivery needs retry: %w", err)}
	}
	return nil
}

const maxPendingAudit = 1000

const MaxPendingAudit = maxPendingAudit

var ErrAuditBacklog = errors.New("audit backlog is full (1000 pending events); release the audit log file lock, run obit reap to deliver pending audit, then retry")

func RequireAuditSpace(st *State) error {
	count := PendingAuditCount(st)
	if count >= maxPendingAudit {
		return ErrAuditBacklog
	}
	return nil
}

func QueueAudit(st *State, event Event) {
	st.PendingAudit = append(st.PendingAudit, event)
}
