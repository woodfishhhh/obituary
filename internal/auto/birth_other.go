//go:build !windows && !darwin && !linux

package auto

import "time"

// birthTime is unavailable here, so nothing counts as created by the agent.
func birthTime(string) (time.Time, bool) { return time.Time{}, false }
