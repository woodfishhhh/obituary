package hook

import "testing"

func TestOpenCodeBridge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testPluginBridge(t, "opencode")
}
