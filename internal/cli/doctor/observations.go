package doctor

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

// queueGrace is how long a request may wait before the queue counts as stuck.
const queueGrace = time.Minute

func (lang diagnostics) diagnoseHookQueue(home string, now time.Time) DoctorCheck {
	status, err := (ledger.Store{Home: home}).InspectObservations()
	c := DoctorCheck{Name: "hook-queue", Status: "PASS", Queue: &status}
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	age := time.Duration(0)
	if !status.Oldest.IsZero() {
		age = max(0, now.Sub(status.Oldest)).Round(time.Second)
	}
	c.Detail = fmt.Sprintf(lang.Text("cli.doctor.oldest_dead_invalid", nil), lang.Count("cli.count.pending_observation", status.Pending), age, status.Dead, status.Invalid, lang.Count("cli.count.incomplete_temporary_file", status.Partial))
	// A resident acknowledges a hook once the request is on disk and registers it
	// moments later, so a young queue is the normal state, not a problem.
	if status.Pending > 0 && age >= queueGrace || status.Dead+status.Invalid+status.Partial > 0 {
		c.Status = "WARN"
	}
	if status.Dead > 0 {
		c.Detail += lang.Text("cli.doctor.failed_requests_are_retained_in_observations_dead", nil)
	}
	return c
}
