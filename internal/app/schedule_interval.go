package app

import "time"

// FallbackInterval is used only by the independent --once maintenance job.
// Resident.Run keeps its own five-minute reconciliation cadence.
const FallbackInterval = time.Hour

// ScheduleInterval changes the independent fallback only after the desktop
// supervisor is actually registered for this data directory.
func ScheduleInterval(home string) time.Duration {
	if DesktopSession() && ResidentEnabled(home) {
		return FallbackInterval
	}
	return Interval
}
