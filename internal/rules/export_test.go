package rules

import "testing"

// SetWalkLimit lowers the scan budget for external tests without large fixtures.
func SetWalkLimit(t *testing.T, limit int) {
	t.Helper()
	before := walkLimit
	walkLimit = limit
	t.Cleanup(func() { walkLimit = before })
}
