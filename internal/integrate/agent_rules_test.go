package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentRulesPreserveUserText(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"", "\xef\xbb\xbf", "\xef\xbb\xbf# User\r\nKeep spaces.  \r\n"} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, "rules.md")
			writeIntegrationFile(t, path, prefix)
			ch, err := planAgentRule(s, path, false)
			if err != nil {
				t.Fatal(err)
			}
			if got := read(t, path); got != prefix {
				t.Fatal("plan wrote rules")
			}
			if err := Apply([]Change{ch}); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(read(t, path), prefix) {
				t.Fatal("user bytes changed")
			}
			ch, err = planAgentRule(s, path, false)
			if err != nil || ch.Action != "unchanged" {
				t.Fatalf("repeat: %+v %v", ch, err)
			}
			ch, err = planAgentRule(s, path, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply([]Change{ch}); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(prefix, "User") && !strings.HasPrefix(read(t, path), prefix) {
				t.Fatal("uninstall changed user text")
			}
		})
	}
}

func TestAgentRulesRefuseAmbiguousMarkersAndLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, content := range []string{blockBegin, blockEnd, blockBegin + blockEnd + blockBegin + blockEnd, "text " + blockBegin + "\nbody\n" + blockEnd, "\xff", "x\x00"} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, "rules")
		writeIntegrationFile(t, path, content)
		for _, remove := range []bool{false, true} {
			if _, err := planAgentRule(s, path, remove); err == nil {
				t.Fatalf("accepted %q", content)
			}
		}
		if read(t, path) != content {
			t.Fatal("bad rules changed")
		}
	}
	s := Scope{Project: t.TempDir()}
	outside := t.TempDir()
	makeIntegrationJunction(t, filepath.Join(s.Project, "linked"), outside)
	if _, err := planAgentRule(s, filepath.Join(s.Project, "linked", "rules"), false); err == nil {
		t.Fatal("followed external junction")
	}
}

func TestPluginPayloadKeepsGuardAndQuotesExecutable(t *testing.T) {
	t.Parallel()
	for _, ext := range []string{".ts", ".py"} {
		s := Scope{Project: t.TempDir(), GuardRM: true}
		path := filepath.Join(s.Project, "plugin"+ext)
		first, err := pluginPayload(s, path, []byte("__OBIT_SETTINGS__"), testenv.NativeFixture(`C:\Program Files\obit.exe`))
		if err != nil {
			t.Fatal(err)
		}
		writeIntegrationFile(t, path, string(first))
		s.GuardRM = false
		next, err := pluginPayload(s, path, []byte("__OBIT_SETTINGS__"), testenv.NativeFixture(`D:\new\obit.exe`))
		if err != nil || !bytes.Contains(next, []byte("true")) || bytes.Contains(next, []byte("Program Files")) {
			t.Fatalf("refresh: %s %v", next, err)
		}
		if _, err := pluginPayload(s, path, []byte("__OBIT_SETTINGS__"), "obit.exe"); err == nil {
			t.Fatal("relative executable accepted")
		}
	}
}

func checkAgentPlanCycle(t *testing.T, agent string, s Scope, plugin, rule string) {
	t.Helper()
	commands := Commands{Executable: testenv.NativeFixture(`C:\Program Files\Obituary\obit.exe`)}
	s.GuardRM = agent != "roo" && agent != "windsurf" && (agent != "hermes" || s.Global)
	before := "\xef\xbb\xbf# My rules\r\nKeep this order.  \r\n"
	if rule != "" {
		writeIntegrationFile(t, rule, before)
	}
	plan, err := PlanAgent(agent, s, commands, false)
	if err != nil {
		t.Fatal(err)
	}
	if plugin != "" {
		if _, err := os.Stat(plugin); !os.IsNotExist(err) {
			t.Fatal("dry-run created plugin")
		}
	}
	if rule != "" && read(t, rule) != before {
		t.Fatal("dry-run changed rules")
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	plan, err = PlanAgent(agent, s, commands, false)
	if err != nil || Pending(plan) {
		t.Fatalf("not idempotent: %+v %v", plan, err)
	}
	s.GuardRM = false
	commands.Executable = testenv.NativeFixture(`D:\New Path\obit.exe`)
	plan, err = PlanAgent(agent, s, commands, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	if plugin != "" {
		text := read(t, plugin)
		if !strings.Contains(text, "New Path") || (!strings.Contains(text, `"guard":true`) && !strings.Contains(text, `\"guard\":true`)) {
			t.Fatalf("plugin refresh lost guard/path: %s", text)
		}
	}
	plan, err = PlanAgent(agent, s, commands, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	if plugin != "" {
		if _, err := os.Stat(plugin); !os.IsNotExist(err) {
			t.Fatal("plugin remains")
		}
	}
	if rule != "" && !strings.HasPrefix(read(t, rule), before) {
		t.Fatal("uninstall changed user rules")
	}
	plan, err = PlanAgent(agent, s, commands, true)
	if err != nil || Pending(plan) {
		t.Fatalf("repeat uninstall: %+v %v", plan, err)
	}
}

func checkEditedPlugin(t *testing.T, agent string, s Scope, plugin string) {
	t.Helper()
	commands := Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}
	plan, err := PlanAgent(agent, s, commands, false)
	applyIntegration(t, plan, err)
	user := read(t, plugin) + "\n# user edit\n"
	writeIntegrationFile(t, plugin, user)
	if _, err := PlanAgent(agent, s, commands, false); err == nil {
		t.Fatal("edited plugin overwritten")
	}
	plan, err = PlanAgent(agent, s, commands, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil {
		t.Fatal("edited plugin not reported")
	}
	if read(t, plugin) != user {
		t.Fatal("user plugin changed")
	}
}

func checkMCPPlanCycle(t *testing.T, planner func(Scope, string, bool) ([]Change, error), s Scope, path string, format ConfigFormat, container []string, initial string) {
	t.Helper()
	first := testenv.NativeFixture(`C:\Program Files\obit.exe`)
	second := testenv.NativeFixture(`D:\new\obit.exe`)
	plan, err := planner(s, first, false)
	if err != nil {
		t.Fatal(err)
	}
	if initial != "" && read(t, path) != initial {
		t.Fatal("MCP plan wrote config")
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	plan, err = planner(s, first, false)
	if err != nil || Pending(plan) {
		t.Fatalf("MCP not idempotent: %+v %v", plan, err)
	}
	plan, err = planner(s, second, false)
	applyIntegration(t, plan, err)
	data, err := configJSON([]byte(read(t, path)), format)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := configMember(data, append(append([]string{}, container...), "obituary"))
	if err != nil || !bytes.Contains(entry, []byte("new")) {
		t.Fatalf("MCP path stale: %s %v", entry, err)
	}
	plan, err = planner(s, second, true)
	applyIntegration(t, plan, err)
	data, err = configJSON([]byte(read(t, path)), format)
	if err != nil {
		t.Fatal(err)
	}
	entry, err = configMember(data, append(append([]string{}, container...), "obituary"))
	if err != nil || entry != nil {
		t.Fatalf("MCP removal: %s %v", entry, err)
	}
	if strings.HasPrefix(initial, "\xef\xbb\xbf") && !strings.HasPrefix(read(t, path), "\xef\xbb\xbf") {
		t.Fatal("MCP BOM lost")
	}
	if strings.Contains(initial, "user comment") && !strings.Contains(read(t, path), "user comment") {
		t.Fatal("MCP comment lost")
	}
	plan, err = planner(s, second, true)
	if err != nil || Pending(plan) {
		t.Fatalf("MCP removal not idempotent: %+v %v", plan, err)
	}
}

func TestAgentPlansRejectExternalLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"cline", "kilocode", "roo"} {
		t.Run(agent, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			sub := map[string]string{"cline": ".cline", "kilocode": ".kilo", "roo": ".roo"}[agent]
			makeIntegrationJunction(t, filepath.Join(root, sub), outside)
			if _, err := PlanAgent(agent, Scope{Project: root}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
				t.Fatal("outside junction accepted")
			}
		})
	}
	for _, agent := range []string{"hermes", "cline", "kilocode", "roo"} {
		t.Run(agent+"-global", func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			sub := "plugins"
			if agent == "kilocode" {
				sub = "plugin"
			}
			if agent == "roo" {
				sub = "rules"
				root = filepath.Join(root, ".roo")
				t.Setenv("OBIT_USER_HOME", filepath.Dir(root))
			}
			t.Setenv("HERMES_HOME", root)
			t.Setenv("CLINE_DIR", root)
			t.Setenv("KILO_CONFIG_DIR", root)
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			makeIntegrationJunction(t, filepath.Join(root, sub), outside)
			if _, err := PlanAgent(agent, Scope{Global: true}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
				t.Fatal("global outside junction accepted")
			}
		})
	}
}

func TestAgentPlansRejectLinkedScopeRoots(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"hermes", "cline", "kilocode", "roo", "windsurf"} {
		t.Run(agent, func(t *testing.T) {
			root := t.TempDir()
			link := filepath.Join(root, "scope")
			makeIntegrationJunction(t, link, t.TempDir())
			if _, err := PlanAgent(agent, Scope{Project: link}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
				t.Fatal("linked project root accepted")
			}
			if _, err := PlanAgent(agent, Scope{Project: "relative"}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
				t.Fatal("relative project root accepted")
			}
			if agent == "hermes" || agent == "cline" || agent == "kilocode" {
				t.Setenv("HERMES_HOME", link)
				t.Setenv("CLINE_DIR", link)
				t.Setenv("KILO_CONFIG_DIR", link)
				if _, err := PlanAgent(agent, Scope{Global: true}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
					t.Fatal("linked global root accepted")
				}
			}
		})
	}
}

func TestAgentApplyRejectsScopeRootReplacedByJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, agent := range []string{"hermes", "cline", "kilocode", "roo", "windsurf"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			root := filepath.Join(parent, "scope")
			outside := t.TempDir()
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			plan, err := PlanAgent(agent, Scope{Project: root}, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(root, filepath.Join(parent, "original")); err != nil {
				t.Fatal(err)
			}
			makeIntegrationJunction(t, root, outside)
			if err := Apply(plan); err == nil {
				t.Fatal("scope root replacement was followed")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatalf("outside was touched: %v %v", entries, err)
			}
		})
	}
}

func TestAgentMCPSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"hermes", "cline", "kilocode", "roo"} {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HERMES_HOME", dir)
			t.Setenv("CLINE_DATA_DIR", dir)
			t.Setenv("KILO_CONFIG_DIR", dir)
			a, _ := LookupAgent(agent)
			s := Scope{Global: agent != "roo", Project: dir}
			plan, err := a.PlanMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false)
			if err != nil {
				t.Fatal(err)
			}
			path := plan[0].Path
			writeIntegrationFile(t, path, "[")
			if _, err := a.PlanMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false); err == nil {
				t.Fatal("bad config accepted")
			}
			if read(t, path) != "[" {
				t.Fatal("bad config changed")
			}
			writeIntegrationFile(t, path, "")
			plan, err = a.PlanMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false)
			if err != nil {
				t.Fatal(err)
			}
			writeIntegrationFile(t, path, "concurrent change")
			if err := Apply(plan); err == nil {
				t.Fatal("MCP race accepted")
			}
		})
	}
	if _, err := PlanWindsurfSkill(Scope{Global: true}, false); err == nil {
		t.Fatal("claimed an unverified user Skill directory")
	}
}

func checkAgentPreflight(t *testing.T, agent string, s Scope) {
	t.Helper()
	plan, err := PlanAgent(agent, s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false)
	if err != nil {
		t.Fatal(err)
	}
	path := plan[0].Path
	writeIntegrationFile(t, path, "concurrent editor")
	if err := Apply(plan); err == nil {
		t.Fatal("concurrent edit accepted")
	}
	if read(t, path) != "concurrent editor" {
		t.Fatal("concurrent content lost")
	}
	for _, ch := range plan[1:] {
		if ch.Action == "create" {
			if _, err := os.Stat(ch.Path); !os.IsNotExist(err) {
				t.Fatalf("partial write: %s", ch.Path)
			}
		}
	}
}
