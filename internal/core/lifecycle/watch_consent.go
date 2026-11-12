package lifecycle

import (
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func explicitlyApproved(p WatchPolicy) bool { return ledger.ExplicitlyApproved(p) }

// Scope exclusions change the transaction revision but not lifespan/age.
// Removing an exclusion is itself an explicit user decision.
func reviseScope(p *WatchPolicy) {
	approved := explicitlyApproved(*p)
	p.Rev++
	if approved {
		p.ApprovedRev = p.Rev
	}
}

func proposeWatch(st *State, p WatchPolicy, now time.Time) { ledger.ProposeWatch(st, p, now) }
