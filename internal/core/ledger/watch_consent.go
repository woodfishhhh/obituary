package ledger

import (
	"slices"
	"time"
)

func ExplicitlyApproved(p WatchPolicy) bool {
	return p.Consent == ConsentApproved && p.Rev > 0 && p.ApprovedRev == p.Rev
}

// A v11 approved policy may have come from v10's implicit first-seen consent.
// Neither a matching lifespan nor an audit-log absence proves a user decision.
func migrateWatchConsent(st *State) []Event {
	var events []Event
	for i := range st.Watches {
		p := &st.Watches[i]
		if p.Consent != ConsentApproved || st.Version >= 12 && ExplicitlyApproved(*p) {
			continue
		}
		p.Consent, p.ApprovedRev, p.Rev = "pending", 0, p.Rev+1
		st.Proposals = slices.DeleteFunc(st.Proposals, func(old Proposal) bool { return old.WatchID == p.ID })
		ProposeWatch(st, *p, time.Now())
		events = append(events, Event{Time: time.Now(), Action: "watch-consent-pending", ID: p.ID, Path: p.Path,
			Detail: "legacy policy requires explicit approval; existing child lifespans are paused"})
	}
	return events
}

func ProposeWatch(st *State, p WatchPolicy, now time.Time) {
	proposal := WatchProposal(p, st.NextID) // carries the structured WatchReview
	proposal.CreatedAt = now
	st.Proposals = append(st.Proposals, proposal)
	st.NextID++
}
