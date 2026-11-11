package integrate

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClineInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "global"}[global], func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".cline")
			t.Setenv("CLINE_DIR", dir)
			checkAgentPlanCycle(t, "cline", Scope{Global: global, Project: root}, filepath.Join(dir, "plugins", "obituary.ts"), filepath.Join(dir, "rules", "obituary.md"))
		})
	}
	s := Scope{Project: t.TempDir()}
	writeIntegrationFile(t, filepath.Join(s.Project, ".clinerules"), "legacy rules\n")
	checkAgentPlanCycle(t, "cline", s, filepath.Join(s.Project, ".cline", "plugins", "obituary.ts"), filepath.Join(s.Project, ".clinerules"))
}

func TestClinePreservesPluginAndRefusesRaces(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	checkEditedPlugin(t, "cline", s, filepath.Join(s.Project, ".cline", "plugins", "obituary.ts"))
	checkAgentPreflight(t, "cline", Scope{Project: t.TempDir()})
}

func TestClineMCP(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLINE_DATA_DIR", dir)
	path := filepath.Join(dir, "settings", "cline_mcp_settings.json")
	initial := "\xef\xbb\xbf" + `{"theme":"keep","mcpServers":{"user":{"command":"user.exe"}}}`
	writeIntegrationFile(t, path, initial)
	checkMCPPlanCycle(t, PlanClineMCP, Scope{Global: true}, path, ConfigJSON, []string{"mcpServers"}, initial)
	if _, err := PlanClineMCP(Scope{Project: t.TempDir()}, `C:\tools\obit.exe`, false); err == nil {
		t.Fatal("project MCP claimed installed")
	}
}

func TestClinePluginRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	checkJSPlugin(t, "cline", clinePlugin)
}

func checkJSPlugin(t *testing.T, agent string, source []byte) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "obituary.ts")
	payload, err := pluginPayload(Scope{Project: dir, GuardRM: true}, path, source, `C:\Program Files\obit.exe`)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, string(payload))
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is unavailable")
	}
	cmd := exec.Command(node, "--experimental-vm-modules", filepath.Join("plugins", agent, "plugin_test.mjs"), path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s plugin: %v\n%s", agent, err, out)
	}
	t.Log(string(out))
}
