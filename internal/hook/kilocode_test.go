package hook

import "testing"

func TestKiloCodeBridge(t *testing.T) {
	checkPluginBridge(t, "kilocode", true)
}

func TestKiloCodeLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	checkPluginLifecycle(t, "kilocode", map[string]string{
		"session.created": "SessionStart", "session.idle": "Stop", "session.deleted": "SessionEnd",
	})
}

func TestKiloCodeObservationOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	checkPluginDeferredObservation(t, "kilocode")
}
