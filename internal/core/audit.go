package core

import (
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

const ActionAdded = ledger.ActionAdded
const ActionUpdated = ledger.ActionUpdated
const ActionExtended = ledger.ActionExtended
const ActionCanceled = ledger.ActionCanceled
const ActionRecycled = ledger.ActionRecycled
const ActionVanished = ledger.ActionVanished
const ActionReplaced = ledger.ActionReplaced
const ActionRefused = ledger.ActionRefused
const ActionRetry = ledger.ActionRetry
const ActionSkipped = ledger.ActionSkipped

type Event = ledger.Event

type AuditError = ledger.AuditError

// Only audit failures mean that an Update committed despite its error.
func commitFailed(err error) bool {
	return err != nil && !OnlyAuditErrors(err)
}

func OnlyAuditErrors(err error) bool { return ledger.OnlyAuditErrors(err) }
