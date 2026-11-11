package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenClawStagingRequiresManualActivation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "global", false: "project"}[global], func(t *testing.T) {
			s, dir := pluginScope(t, "openclaw", global)
			config := filepath.Join(t.TempDir(), "openclaw.json")
			t.Setenv("OPENCLAW_CONFIG_PATH", config)
			original := "\xef\xbb\xbf{ // keep comment\n plugins: {allow:['user-plugin']}, broken: 'JSON5 preserved' }"
			writeIntegrationFile(t, config, original)
			cmd := Commands{Executable: `C:\Program Files\obit.exe`}
			plan, err := PlanOpenClaw(s, cmd, false)
			if err != nil {
				t.Fatal(err)
			}
			if !Pending(plan) || PreservedError(plan) == nil {
				t.Fatal("manual activation was reported complete")
			}
			if _, err := os.Stat(filepath.Join(dir, "extensions", "obituary", "index.ts")); !os.IsNotExist(err) {
				t.Fatal("dry-run wrote")
			}
			if err := Apply(plan); err == nil || !strings.Contains(err.Error(), "manual activation") {
				t.Fatalf("manual status: %v", err)
			}
			if read(t, config) != original {
				t.Fatal("JSON5 changed")
			}
			plan, err = PlanOpenClaw(s, cmd, false)
			if err != nil || Pending(plan) {
				t.Fatalf("staging not idempotent %v", err)
			}
			cmd.Executable = `C:\new\obit.exe`
			plan, err = PlanOpenClaw(s, cmd, false)
			if err != nil {
				t.Fatal(err)
			}
			if err = Apply(plan); err == nil {
				t.Fatal("activation falsely acknowledged")
			}
			if !strings.Contains(read(t, filepath.Join(dir, "extensions", "obituary", "runtime.json")), "new") {
				t.Fatal("runtime not refreshed")
			}
			plan, err = PlanOpenClaw(s, cmd, true)
			if err != nil || Pending(plan) || PreservedError(plan) == nil {
				t.Fatal("unsafe automatic removal")
			}
			if _, err = os.Stat(filepath.Join(dir, "extensions", "obituary", "index.ts")); err != nil {
				t.Fatal("removed registered plugin")
			}
			mcp, err := PlanOpenClawMCP(s, cmd.Executable, false)
			if err != nil || PreservedError(mcp) == nil || !strings.Contains(mcp[0].Detail, "mcp.servers.obituary") {
				t.Fatal("missing manual MCP steps")
			}
		})
	}
}

func TestOpenClawPluginProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	runPluginNodeTests(t, "openclaw")
}

func TestOpenClawPluginDeferredContext(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	testPluginContext(t, "openclaw")
}
