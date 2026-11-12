package lifecycle

import (
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

const maxPendingAudit = ledger.MaxPendingAudit

var errAuditBacklog = ledger.ErrAuditBacklog

func requireAuditSpace(st *State) error { return ledger.RequireAuditSpace(st) }

func pendingAuditCount(st *State) int { return ledger.PendingAuditCount(st) }

// Each public batch owns this hint, shared by its nested service copies. Normal
// ledger reads/writes discover pending work, so an empty batch needs no extra
// ledger read or delivery lock. This is not a cache across operations/processes.
func (s *Service) auditBatch(dryRun bool) (*Service, func() error) {
	if dryRun {
		return s, func() error { return nil }
	}
	store, finish, created := s.Store.BeginAuditBatch()
	if !created {
		return s, finish
	}
	s = s.WithContext(s.Store.Context())
	s.Store = store
	return s, finish
}

// Queue the event inside the business Store.Update. This is the transactional
// outbox pattern (microservices.io/patterns/data/transactional-outbox.html).
// Existing JSON transactions and audit locks supply persistence/serialization;
// a SQL/broker outbox library cannot join this file-backed transaction.
func queueAudit(st *State, event Event) { ledger.QueueAudit(st, event) }
