package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraeInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"trae", "trae-cn"} {
		for _, global := range []bool{false, true} {
			t.Run(agent+map[bool]string{true: "-global", false: "-project"}[global], func(t *testing.T) {
				home, project := t.TempDir(), t.TempDir()
				t.Setenv("OBIT_USER_HOME", home)
				s := Scope{Global: global, Project: project}
				root := filepath.Join(project, ".trae")
				if global {
					root = filepath.Join(home, "."+agent)
				}
				path := filepath.Join(root, "hooks.json")
				input := "\xef\xbb\xbf" + `{"theme":"dark","hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo user && obit hook ` + agent + `"}]}]},"model":"user"}`
				writeIntegrationFile(t, path, input)
				commands := Commands{Native: `"C:/space & tools/obit.exe" hook ` + agent}
				plan, err := PlanAgent(agent, s, commands, false)
				if err != nil || read(t, path) != input {
					t.Fatalf("dry-run: %v", err)
				}
				applyIntegration(t, plan, nil)
				installed := read(t, path)
				if !strings.HasPrefix(installed, "\xef\xbb\xbf") || !strings.Contains(installed, `& \"C:/space & tools/obit.exe\"`) || strings.Index(installed, `"theme"`) > strings.Index(installed, `"model"`) {
					t.Fatal(installed)
				}
				for _, unexpected := range []string{"PreToolUse", "PostToolUse", "SessionEnd"} {
					if strings.Contains(installed, unexpected) {
						t.Fatal("registered unsupported event", unexpected)
					}
				}
				plan, err = PlanAgent(agent, s, commands, false)
				if err != nil || Pending(plan) {
					t.Fatalf("idempotence: %+v %v", plan, err)
				}
				commands.Native = `"D:/new location/obit.exe" hook ` + agent
				plan, err = PlanAgent(agent, s, commands, false)
				applyIntegration(t, plan, err)
				if strings.Contains(read(t, path), "space & tools") || !strings.Contains(read(t, path), "new location") {
					t.Fatal("command not refreshed")
				}
				plan, err = PlanAgent(agent, s, Commands{}, true)
				applyIntegration(t, plan, err)
				got := read(t, path)
				if strings.Contains(got, "new location") || !strings.Contains(got, "echo user && obit hook "+agent) || !strings.Contains(got, `"theme": "dark"`) || read(t, path+".bak") != input {
					t.Fatal("uninstall changed user config", got)
				}
				plan, err = PlanAgent(agent, s, Commands{}, true)
				if err != nil || Pending(plan) {
					t.Fatalf("repeat uninstall: %v", err)
				}
			})
		}
	}
}

func TestTraeVariantsDoNotInstallDuplicateProjectHooks(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	plan, err := PlanTrae(s, Commands{Native: "C:/tools/obit.exe hook trae"}, false)
	applyIntegration(t, plan, err)
	path := filepath.Join(s.Project, ".trae", "hooks.json")
	before := read(t, path)
	if _, err := PlanTraeCN(s, Commands{Native: "C:/tools/obit.exe hook trae-cn"}, false); err == nil {
		t.Fatal("second variant installed over shared project hooks")
	}
	plan, err = PlanTraeCN(s, Commands{}, true)
	applyIntegration(t, plan, err)
	if read(t, path) != before {
		t.Fatal("uninstalled another variant's hooks")
	}
}

func TestTraeChineseDiscoveryAndPreflight(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	s, commands := Scope{Global: true}, Commands{Native: "C:/tools/obit.exe hook trae"}
	plan, err := PlanTrae(s, commands, false)
	if err != nil || len(plan) != 1 {
		t.Fatalf("invented absent CN installation: %+v %v", plan, err)
	}
	cn := filepath.Join(home, ".trae-cn", "hooks.json")
	writeIntegrationFile(t, cn, "broken")
	if _, err := PlanTrae(s, commands, false); err == nil {
		t.Fatal("invalid CN config accepted")
	}
	writeIntegrationFile(t, cn, "")
	plan, err = PlanTrae(s, commands, false)
	if err != nil || len(plan) != 2 {
		t.Fatalf("CN discovery: %+v %v", plan, err)
	}
	writeIntegrationFile(t, cn, "{}")
	if err := Apply(plan); err == nil {
		t.Fatal("concurrent CN edit accepted")
	}
	if _, err := os.Stat(filepath.Join(home, ".trae", "hooks.json")); !os.IsNotExist(err) {
		t.Fatal("partial write before CN preflight")
	}
	plan, err = PlanTrae(s, commands, false)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, cn), "C:/tools/obit.exe hook trae-cn") {
		t.Fatal("CN adapter identity lost")
	}
}

func TestTraeRejectsInvalidConfigAndLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, input := range []string{"{", `{"version":2}`, `{"version":null}`, `{"hooks":[]}`, `{"hooks":{"Stop":{}}}`} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, ".trae", "hooks.json")
		writeIntegrationFile(t, path, input)
		for _, uninstall := range []bool{false, true} {
			if _, err := PlanTrae(s, Commands{Native: "C:/tools/obit.exe hook trae"}, uninstall); err == nil {
				t.Fatal("accepted invalid config", input)
			}
		}
		if read(t, path) != input {
			t.Fatal("planner wrote config")
		}
	}
	s := Scope{Project: t.TempDir()}
	outside := t.TempDir()
	makeIntegrationJunction(t, filepath.Join(s.Project, ".trae"), outside)
	if _, err := PlanTrae(s, Commands{Native: "C:/tools/obit.exe hook trae"}, false); err == nil {
		t.Fatal("external link accepted")
	}
	s.GuardRM = true
	if _, err := PlanTrae(s, Commands{Native: "C:/tools/obit.exe hook trae"}, false); err == nil {
		t.Fatal("unsupported guard accepted")
	}
}

func TestTraeMCPLifecycleAndManualSteps(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, ".trae", "mcp.json")
	input := "\xef\xbb\xbf" + `{"theme":"keep","mcpServers":{"other":{"command":"other"}}}`
	writeIntegrationFile(t, path, input)
	plan, err := PlanTraeMCP(s, "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
	got := strings.Replace(read(t, path), `"args": [`, `"env": {"USER":"keep"}, "args": [`, 1)
	writeIntegrationFile(t, path, got)
	plan, err = PlanTraeMCP(s, "D:/new/obit.exe", false)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), `"USER"`) {
		t.Fatal("MCP env lost")
	}
	plan, err = PlanTraeMCP(s, "D:/new/obit.exe", false)
	if err != nil || Pending(plan) {
		t.Fatalf("MCP not idempotent: %v", err)
	}
	plan, err = PlanTraeMCP(s, "", true)
	applyIntegration(t, plan, err)
	if strings.Contains(read(t, path), `"obituary"`) || !strings.Contains(read(t, path), `"other"`) {
		t.Fatal("MCP uninstall")
	}
	for _, uninstall := range []bool{false, true} {
		if _, err := PlanTraeMCP(Scope{Global: true}, "C:/tools/obit.exe", uninstall); err == nil || !strings.Contains(err.Error(), "Settings > MCP") {
			t.Fatalf("missing manual global instructions: %v", err)
		}
	}
	if _, err := PlanTraeMCP(s, "C:/space dir/obit.exe", false); err == nil {
		t.Fatal("unverified spaced MCP command accepted")
	}
}
