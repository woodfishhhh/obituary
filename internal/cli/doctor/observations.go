package doctor

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

func diagnoseHookQueue(home string, now time.Time) DoctorCheck {
	status, err := (core.Store{Home: home}).InspectObservations()
	c := DoctorCheck{Name: "hook-queue", Status: "PASS", Queue: &status}
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	age := time.Duration(0)
	if !status.Oldest.IsZero() {
		age = max(0, now.Sub(status.Oldest)).Round(time.Second)
	}
	c.Detail = fmt.Sprintf("%s; oldest %s; %d dead; %d invalid; %s", textutil.Count(status.Pending, "pending observation"), age, status.Dead, status.Invalid, textutil.Count(status.Partial, "incomplete temporary file"))
	if status.Pending+status.Dead+status.Invalid+status.Partial > 0 {
		c.Status = "WARN"
	}
	if status.Dead > 0 {
		c.Detail += "; failed requests are retained in observations/dead"
	}
	return c
}
