package ledger

// Graveyard actions and burial reasons.
const (
	ActionBuried          = "buried"      // moved into the graveyard
	ActionResurrected     = "resurrected" // moved back out of the graveyard
	ActionPurged          = "purged"      // deleted for good after its retention
	ActionLost            = "lost"        // its grave was already empty
	ActionRecoveryPending = "recovery pending"
	ActionNotExecuted     = "not executed"
	ActionBlocked         = "blocked"

	ReasonExpired = "expired" // its lifespan ran out
	ReasonBury    = "bury"    // buried on request (obit bury, intercepted deletes)
)

const ActionAbandoned = "abandoned"
