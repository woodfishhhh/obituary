package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestHermesInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	config := filepath.Join(dir, "config.yaml")
	writeIntegrationFile(t, config, "\xef\xbb\xbf# user comment\nmodel: mine\nplugins:\n  enabled: [other] # enabled comment\n")
	checkAgentPlanCycle(t, "hermes", Scope{Global: true}, filepath.Join(dir, "plugins", "obituary", "__init__.py"), "")
	text := read(t, config)
	for _, keep := range []string{"\xef\xbb\xbf", "# user comment", "model: mine", "other", "# enabled comment"} {
		if !strings.Contains(text, keep) {
			t.Fatalf("lost %q: %s", keep, text)
		}
	}
	s := Scope{Project: t.TempDir()}
	checkAgentPlanCycle(t, "hermes", s, "", filepath.Join(s.Project, "AGENTS.md"))
	s.GuardRM = true
	if _, err := PlanHermes(s, Commands{}, false); err == nil {
		t.Fatal("project guard silently installed")
	}
}

func TestHermesPreservesPluginAndRegistration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	checkEditedPlugin(t, "hermes", Scope{Global: true}, filepath.Join(dir, "plugins", "obituary", "__init__.py"))
	if strings.Contains(read(t, filepath.Join(dir, "config.yaml")), "- obituary") {
		t.Fatal("plugin remained enabled")
	}
}

func TestHermesRejectsBadConfig(t *testing.T) {
	for _, bad := range []string{"plugins: [", "plugins: nope", "plugins:\n  enabled: null", "plugins:\n  enabled: [12]"} {
		dir := t.TempDir()
		t.Setenv("HERMES_HOME", dir)
		path := filepath.Join(dir, "config.yaml")
		writeIntegrationFile(t, path, bad)
		if _, err := PlanHermes(Scope{Global: true}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
			t.Fatalf("accepted %q", bad)
		}
		if read(t, path) != bad {
			t.Fatal("bad config changed")
		}
	}
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	checkAgentPreflight(t, "hermes", Scope{Global: true})
}

func TestHermesMCP(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	path := filepath.Join(dir, "config.yaml")
	for _, initial := range []string{"", "\xef\xbb\xbf# user\nmodel: keep\nmcp_servers:\n  other:\n    command: user.exe\n"} {
		writeIntegrationFile(t, path, initial)
		checkMCPPlanCycle(t, PlanHermesMCP, Scope{Global: true}, path, ConfigYAML, []string{"mcp_servers"}, initial)
	}
	if _, err := PlanHermesMCP(Scope{Project: t.TempDir()}, testenv.NativeFixture(`C:\tools\obit.exe`), false); err == nil {
		t.Fatal("project MCP claimed installed")
	}
}

func TestHermesCombinedMCPInstallation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	path := filepath.Join(dir, "config.yaml")
	writeIntegrationFile(t, path, "# keep\nplugins:\n  enabled: [other]\n")
	for _, remove := range []bool{false, false, true, true} {
		base, err := PlanHermes(Scope{Global: true}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, remove)
		if err != nil {
			t.Fatal(err)
		}
		ext, err := PlanHermesMCP(Scope{Global: true}, testenv.NativeFixture(`C:\tools\obit.exe`), remove)
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
		text := read(t, path)
		if !strings.Contains(text, "# keep") || !strings.Contains(text, "other") {
			t.Fatal("user config lost")
		}
	}
}

func TestHermesUninstallRestoresAbsentContainers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, containers := range []string{"", "plugins: {}\n", "plugins:\n  enabled: []\n", "mcp_servers: {}\n", "plugins:\n  enabled: [other]\nmcp_servers:\n  other:\n    command: user\n"} {
		t.Run(containers, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HERMES_HOME", root)
			path := filepath.Join(root, "config.yaml")
			original := "# user-sentinel\nmodel: mine\n" + containers
			writeIntegrationFile(t, path, original)
			for _, uninstall := range []bool{false, false, true, true} {
				base, err := PlanHermes(Scope{Global: true}, ownershipCommands("hermes"), uninstall)
				if err != nil {
					t.Fatal(err)
				}
				ext, err := PlanHermesMCP(Scope{Global: true}, testenv.NativeFixture("C:/tools/obit.exe"), uninstall)
				if err != nil {
					t.Fatal(err)
				}
				plan, err := ComposeAgentChanges(append(base, ext...))
				applyIntegration(t, plan, err)
			}
			if got := read(t, path); got != original {
				t.Fatalf("Hermes changed original containers: got %q want %q", got, original)
			}
		})
	}
}

func TestHermesUnownedEnablement(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	path := filepath.Join(dir, "config.yaml")
	writeIntegrationFile(t, path, "plugins:\n  enabled: [obituary]\n")
	for _, remove := range []bool{false, true} {
		if _, err := PlanHermes(Scope{Global: true}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, remove); err == nil {
			t.Fatal("unowned enablement claimed")
		}
	}
}

func TestHermesPluginRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	dir := t.TempDir()
	payload, err := pluginPayload(Scope{Project: dir, GuardRM: true}, filepath.Join(dir, "__init__.py"), hermesPlugin, testenv.NativeFixture(`C:\Program Files\obit.exe`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "__init__.py")
	writeIntegrationFile(t, path, string(payload))
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("Python is unavailable")
	}
	cmd := exec.Command(python, "-B", filepath.Join("plugins", "hermes", "plugin_test.py"), path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python plugin: %v\n%s", err, out)
	}
	t.Log(string(out))
}

func TestHermesPatchObservationIgnoresUnusedPath(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root, e := pluginEngine(t, true)
	neighbor, actual := filepath.Join(root, "untouched.log"), filepath.Join(root, "real.log")
	writeIntegrationFile(t, neighbor, "untouched")
	writeIntegrationFile(t, actual, "host patched here")
	pluginRoundTrip(t, "hermes", root, e, map[string]any{"patch": true})
	pluginSweepKeepsNeighbor(t, e, neighbor)
	if _, err := os.Stat(actual); !os.IsNotExist(err) {
		t.Fatalf("actual patch target not observed: %v", err)
	}
}

func TestHermesPluginDeferredContext(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	var manifest map[string]any
	if err := yaml.Unmarshal(hermesManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"hooks", "provides_hooks"} {
		names, ok := manifest[field].([]any)
		if !ok || !slices.Contains(names, any("pre_llm_call")) {
			t.Fatalf("manifest does not declare context hook: %s", field)
		}
	}
	testPluginContext(t, "hermes")
}

func TestHermesSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "hermes")
}

func TestHermesSkillSharedOwners(t *testing.T) {
	checkSharedSkillOwners(t, Scope{Project: t.TempDir()}, "codex", "hermes")
}
