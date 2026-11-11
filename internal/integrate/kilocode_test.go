package integrate

import (
	"path/filepath"
	"testing"
)

func TestKiloCodePluginDeferredContext(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	testPluginContext(t, "kilocode")
}

func TestKiloCodeInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "global"}[global], func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".kilo")
			t.Setenv("KILO_CONFIG_DIR", dir)
			rule := filepath.Join(root, ".kilocode", "rules", "obituary.md")
			if global {
				rule = filepath.Join(dir, "rules", "obituary.md")
			}
			checkAgentPlanCycle(t, "kilocode", Scope{Global: global, Project: root}, filepath.Join(dir, "plugin", "obituary.ts"), rule)
		})
	}
}

func TestKiloCodePreservesPluginAndRefusesRaces(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	checkEditedPlugin(t, "kilocode", s, filepath.Join(s.Project, ".kilo", "plugin", "obituary.ts"))
	checkAgentPreflight(t, "kilocode", Scope{Project: t.TempDir()})
}

func TestKiloCodeMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		root := t.TempDir()
		dir := filepath.Join(root, ".kilo")
		t.Setenv("KILO_CONFIG_DIR", dir)
		path := filepath.Join(dir, "kilo.jsonc")
		initial := "\xef\xbb\xbf" + "{\n// user comment\n\"model\":\"keep\",\n\"mcp\":{\"other\":{\"type\":\"remote\",\"url\":\"https://example.invalid\"}},\n}"
		writeIntegrationFile(t, path, initial)
		checkMCPPlanCycle(t, PlanKiloCodeMCP, Scope{Global: global, Project: root}, path, ConfigJSONC, []string{"mcp"}, initial)
		writeIntegrationFile(t, filepath.Join(dir, "kilo.json"), "{}")
		if _, err := PlanKiloCodeMCP(Scope{Global: global, Project: root}, `C:\tools\obit.exe`, false); err == nil {
			t.Fatal("ambiguous config accepted")
		}
	}
}

func TestKiloCodeCombinedMCPInstallation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KILO_CONFIG_DIR", dir)
	path := filepath.Join(dir, "kilo.jsonc")
	writeIntegrationFile(t, path, "{\n// user\n\"instructions\":[\"user.md\"]\n}")
	for _, remove := range []bool{false, false, true, true} {
		base, err := PlanKiloCode(Scope{Global: true}, Commands{Executable: `C:\tools\obit.exe`}, remove)
		if err != nil {
			t.Fatal(err)
		}
		ext, err := PlanKiloCodeMCP(Scope{Global: true}, `C:\tools\obit.exe`, remove)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := ComposeAgentChanges(append(base, ext...))
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(plan); err != nil {
			t.Fatal(err)
		}
	}
}

func TestKiloCodePluginRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	checkJSPlugin(t, "kilocode", kiloCodePlugin)
}
