package integrate

import "testing"

func TestOMPInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testPluginLifecycle(t, "omp")
}
