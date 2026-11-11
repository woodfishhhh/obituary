package core

import (
	"context"
	"errors"
	"fmt"
)

const maxPendingAudit = 1000

var errAuditBacklog = errors.New("audit backlog is full (1000 pending events); release the audit log file lock, run obit reap to deliver pending audit, then retry")

func (st *State) requireAuditSpace() error {
	count := st.pendingAuditCount()
	if count >= maxPendingAudit {
		return errAuditBacklog
	}
	return nil
}

func (st *State) pendingAuditCount() int {
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
		if b.PurgeIntent != nil && st.containerIntent(b.ID) == nil {
			count++
		}
	}
	return count
}

// Each public batch owns this hint, shared by its nested service copies. Normal
// ledger reads/writes discover pending work, so an empty batch needs no extra
// ledger read or delivery lock. This is not a cache across operations/processes.
func (s *Service) auditBatch(dryRun bool) (*Service, func() error) {
	if dryRun || s.Store.auditPending != nil {
		return s, func() error { return nil }
	}
	s = s.WithContext(s.Store.Context())
	s.Store.auditPending = new(bool)
	return s, func() error {
		if !*s.Store.auditPending {
			return nil
		}
		store := s.Store
		store.auditPending = nil
		return store.FlushAudit()
	}
}

// Queue the event inside the business Store.Update. This is the transactional
// outbox pattern (microservices.io/patterns/data/transactional-outbox.html).
// Existing JSON transactions and audit locks supply persistence/serialization;
// a SQL/broker outbox library cannot join this file-backed transaction.
func (st *State) queueAudit(event Event) {
	st.PendingAudit = append(st.PendingAudit, event)
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
