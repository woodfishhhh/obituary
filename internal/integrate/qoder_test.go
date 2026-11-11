package integrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const qoderUserConfig = "\xef\xbb\xbf" + `{
  // user comment
  "theme": "dark",
  "hooks": {"PostToolUse": [{"matcher":"Read","hooks":[{"type":"command","command":"user-lint","timeout":41}]}]},
  "permissions": {"deny":["Bash(rm:*)"]},
  "last": true
}`

func qoderTestScope(t *testing.T, agent string, global bool) (Scope, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("QODER_CONFIG_DIR", filepath.Join(home, ".qoder"))
	t.Setenv("QODERCN_CONFIG_DIR", filepath.Join(home, ".qoder-cn"))
	s := Scope{Global: global, Project: t.TempDir()}
	dir, err := qoderDirectory(s, agent)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func qoderTestCommands(agent, folder string) Commands {
	exe := "C:/" + folder + "/obit.exe"
	return Commands{Native: `"` + exe + `" hook ` + agent, Executable: exe}
}

func TestQoderInstallerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, name := range []string{"qoder", "qoder-cn"} {
		for _, global := range []bool{false, true} {
			t.Run(name+map[bool]string{true: "/global", false: "/project"}[global], func(t *testing.T) {
				s, dir := qoderTestScope(t, name, global)
				path := filepath.Join(dir, "settings.json")
				writeIntegrationFile(t, path, qoderUserConfig)
				userRule := filepath.Join(dir, "rules", "user.md")
				metadata := "---\ntrigger: glob\nglob: '**/*.go'\ndescription: 用户规则\n---\nkeep\n"
				writeIntegrationFile(t, userRule, metadata)
				agent, _ := LookupAgent(name)
				commands := qoderTestCommands(name, "tools with space")
				plan, err := agent.Plan(s, commands, false)
				if err != nil || read(t, path) != qoderUserConfig {
					t.Fatal("dry-run changed configuration", err)
				}
				if _, err := os.Stat(filepath.Join(dir, "rules", "obituary.md")); !os.IsNotExist(err) {
					t.Fatal("dry-run wrote rule", err)
				}
				applyIntegration(t, plan, nil)
				plan, err = agent.Plan(s, commands, false)
				if err != nil || Pending(plan) {
					t.Fatal("not idempotent", err, plan)
				}
				plan, err = agent.PlanMCP(s, commands.Executable, false)
				applyIntegration(t, plan, err)
				plan, err = agent.PlanMCP(s, commands.Executable, false)
				if err != nil || Pending(plan) {
					t.Fatal("MCP not idempotent", err)
				}
				commands = qoderTestCommands(name, "new tools")
				plan, err = agent.Plan(s, commands, false)
				applyIntegration(t, plan, err)
				plan, err = agent.PlanMCP(s, commands.Executable, false)
				applyIntegration(t, plan, err)
				installed := read(t, path)
				for _, text := range []string{"\xef\xbb\xbf", "// user comment", "user-lint", "Bash(rm:*)", "new tools"} {
					if !strings.Contains(installed, text) {
						t.Fatal("lost config", text, installed)
					}
				}
				if strings.Index(installed, `"theme"`) > strings.Index(installed, `"last"`) || strings.Contains(installed, "PreToolUse") {
					t.Fatal("order or guard", installed)
				}
				if read(t, userRule) != metadata {
					t.Fatal("user metadata changed")
				}
				plan, err = agent.Plan(s, Commands{}, true)
				applyIntegration(t, plan, err)
				plan, err = agent.PlanMCP(s, "", true)
				applyIntegration(t, plan, err)
				if read(t, userRule) != metadata || strings.Contains(read(t, path), "hook qoder") || strings.Contains(read(t, path), `"obituary"`) {
					t.Fatal("uninstall damaged user or left registration", read(t, path))
				}
				if _, err := os.Stat(filepath.Join(dir, "rules", "obituary.md")); !os.IsNotExist(err) {
					t.Fatal("owned rule remains", err)
				}
			})
		}
	}
}

func TestQoderSharedProjectOwners(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, first := range []string{"qoder", "qoder-cn"} {
		t.Run(first, func(t *testing.T) {
			s, dir := qoderTestScope(t, first, false)
			second := "qoder-cn"
			if first == second {
				second = "qoder"
			}
			for _, name := range []string{first, second} {
				p, err := PlanAgent(name, s, qoderTestCommands(name, "tools"), false)
				applyIntegration(t, p, err)
				p, err = planQoderMCP(s, "C:/tools/obit.exe", name, false)
				applyIntegration(t, p, err)
			}
			path := filepath.Join(dir, "settings.json")
			if strings.Count(read(t, path), "hook qoder") != 5 || strings.Contains(read(t, path), "hook qoder-cn") {
				t.Fatal("duplicate edition callbacks", read(t, path))
			}
			p, err := PlanAgent(first, s, Commands{}, true)
			applyIntegration(t, p, err)
			p, err = planQoderMCP(s, "", first, true)
			applyIntegration(t, p, err)
			if strings.Count(read(t, path), "hook qoder") != 5 || !strings.Contains(read(t, filepath.Join(s.Project, ".mcp.json")), "obituary") {
				t.Fatal("first owner removed shared registration")
			}
			if _, err := os.Stat(filepath.Join(dir, "rules", "obituary.md")); err != nil {
				t.Fatal("first owner removed shared rule", err)
			}
			// The remaining owner may refresh the exe without rewriting rule metadata.
			p, err = PlanAgent(second, s, qoderTestCommands(second, "new tools"), false)
			applyIntegration(t, p, err)
			p, err = PlanAgent(second, s, Commands{}, true)
			applyIntegration(t, p, err)
			p, err = planQoderMCP(s, "", second, true)
			applyIntegration(t, p, err)
			if strings.Contains(read(t, path), "hook qoder") || strings.Contains(read(t, filepath.Join(s.Project, ".mcp.json")), "obituary") {
				t.Fatal("last owner left registration")
			}
		})
	}
}

func TestQoderConfigurationLayers(t *testing.T) {
	s, dir := qoderTestScope(t, "qoder", false)
	lower, local := filepath.Join(dir, "settings.json"), filepath.Join(dir, "settings.local.json")
	writeIntegrationFile(t, lower, qoderUserConfig)
	writeIntegrationFile(t, local, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"user-stop"}]}]},"local":true}`)
	p, err := PlanQoder(s, qoderTestCommands("qoder", "tools"), false)
	applyIntegration(t, p, err)
	if strings.Contains(read(t, lower), "hook qoder") || strings.Count(read(t, local), "hook qoder") != 5 || !strings.Contains(read(t, local), "user-stop") {
		t.Fatal("wrong hook priority")
	}
	hooksOnly := read(t, local)
	withMCP := strings.TrimSuffix(hooksOnly, "}\n") + `,"mcpServers":{"obituary":{"command":"C:/old/obit.exe","args":["mcp"],"env":{"KEEP":"yes"}}}}`
	writeIntegrationFile(t, local, withMCP)
	if _, err := PlanQoderMCP(s, "C:/new/obit.exe", false); err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatal("unowned shared MCP adopted", err)
	}
	// An existing installation from the shared plugin installer records the
	// owner set; Qoder must cooperate with that receipt instead of replacing it.
	writeIntegrationFile(t, local, hooksOnly)
	p, err = planPluginMCP(s, local, "qoder", "C:/old/obit.exe", false)
	applyIntegration(t, p, err)
	writeIntegrationFile(t, local, withMCP)
	p, err = PlanQoderMCP(s, "C:/new/obit.exe", false)
	applyIntegration(t, p, err)
	if !strings.Contains(read(t, local), "C:/new/obit.exe") || !strings.Contains(read(t, local), "KEEP") {
		t.Fatal("local MCP lost fields")
	}
	if _, err := os.Stat(filepath.Join(s.Project, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatal("created shadowed MCP", err)
	}
	writeIntegrationFile(t, filepath.Join(s.Project, ".mcp.json"), `{"mcpServers":{"obituary":{"command":"C:/other/obit.exe","args":["mcp"]}}}`)
	for _, uninstall := range []bool{false, true} {
		if _, err := PlanQoderMCP(s, "C:/tools/obit.exe", uninstall); err == nil || !strings.Contains(err.Error(), "multiple layers") {
			t.Fatal("ambiguous MCP silently changed", err)
		}
	}
}

func TestQoderInstallationBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, name := range []string{"qoder", "qoder-cn", "lingma"} {
		t.Run(name, func(t *testing.T) {
			agent, _ := LookupAgent(name)
			for _, invalid := range []string{"", "\xef\xbb\xbf", "{", "[]", `{"hooks": []}`} {
				s, dir := qoderTestScope(t, name, false)
				path := filepath.Join(dir, "settings.json")
				writeIntegrationFile(t, path, invalid)
				if _, err := agent.Plan(s, qoderTestCommands(name, "tools"), false); err == nil {
					t.Fatal("bad config accepted", invalid)
				}
				if read(t, path) != invalid {
					t.Fatal("bad config changed")
				}
			}
			s, dir := qoderTestScope(t, name, false)
			path := filepath.Join(dir, "settings.json")
			p, err := agent.Plan(s, qoderTestCommands(name, "tools"), false)
			if err != nil {
				t.Fatal(err)
			}
			writeIntegrationFile(t, path, `{"concurrent":true}`)
			if Apply(p) == nil || read(t, path) != `{"concurrent":true}` {
				t.Fatal("stale plan applied")
			}
			s.GuardRM = true
			if _, err := agent.Plan(s, qoderTestCommands(name, "tools"), false); err == nil {
				t.Fatal("unverified guard installed")
			}
			for _, global := range []bool{false, true} {
				s, dir = qoderTestScope(t, name, global)
				makeIntegrationJunction(t, dir, t.TempDir())
				if _, err := agent.Plan(s, qoderTestCommands(name, "tools"), false); err == nil {
					t.Fatal("external link accepted")
				}
			}
		})
	}
}

func TestQoderEditedRuleMetadataPreserved(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, name := range []string{"qoder", "qoder-cn", "lingma"} {
		t.Run(name, func(t *testing.T) {
			s, dir := qoderTestScope(t, name, false)
			p, err := PlanAgent(name, s, qoderTestCommands(name, "tools"), false)
			applyIntegration(t, p, err)
			path := filepath.Join(dir, "rules", "obituary.md")
			edited := "---\ntrigger: manual\nalwaysApply: false\ndescription: user change\n---\n" + read(t, path)
			writeIntegrationFile(t, path, edited)
			if _, err := PlanAgent(name, s, qoderTestCommands(name, "new"), false); err == nil {
				t.Fatal("edited rule claimed")
			}
			p, err = PlanAgent(name, s, Commands{}, true)
			if err != nil || Apply(p) == nil || read(t, path) != edited {
				t.Fatal("edited metadata lost or not reported", err)
			}
			if strings.Contains(read(t, filepath.Join(dir, "settings.json")), "hook ") {
				t.Fatal("edited rule prevented hook removal")
			}
		})
	}
}

func TestQoderMCPConflictAndConcurrency(t *testing.T) {
	s, dir := qoderTestScope(t, "qoder", false)
	path := filepath.Join(s.Project, ".mcp.json")
	for _, bad := range []string{"", "\xef\xbb\xbf", "{", "[]", `{"mcpServers":{"obituary":{"command":"user.exe","args":[]}}}`} {
		writeIntegrationFile(t, path, bad)
		if _, err := PlanQoderMCP(s, "C:/tools/obit.exe", false); err == nil || read(t, path) != bad {
			t.Fatal("bad MCP accepted/changed", bad, err)
		}
	}
	writeIntegrationFile(t, path, `{"mcpServers":{}}`)
	p, err := PlanQoderMCP(s, "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	// A newly added higher-priority file must invalidate the whole plan.
	writeIntegrationFile(t, filepath.Join(dir, "settings.local.json"), `{"mcpServers":{"obituary":{"command":"user.exe"}}}`)
	if Apply(p) == nil || read(t, path) != `{"mcpServers":{}}` {
		t.Fatal("new higher-priority config ignored")
	}
}

func TestQoderHookHandlerTimeout(t *testing.T) {
	s, dir := qoderTestScope(t, "qoder", false)
	p, err := PlanQoder(s, qoderTestCommands("qoder", "tools with spaces"), false)
	applyIntegration(t, p, err)
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string
				Timeout int
			}
		}
	}
	if err := json.Unmarshal([]byte(read(t, filepath.Join(dir, "settings.json"))), &settings); err != nil {
		t.Fatal(err)
	}
	for _, groups := range settings.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if handler.Timeout != 5 || !OwnedHookCommand(handler.Command, "qoder", nil) {
					t.Fatal("handler", handler)
				}
			}
		}
	}
}

func TestQoderSharesPluginMCPOwnership(t *testing.T) {
	s, _ := qoderTestScope(t, "qoder", false)
	path := filepath.Join(s.Project, ".mcp.json")
	p, err := planPluginMCP(s, path, "pi", "C:/tools/obit.exe", false)
	applyIntegration(t, p, err)
	p, err = PlanQoderMCP(s, "C:/tools/obit.exe", false)
	applyIntegration(t, p, err)
	p, err = PlanQoderMCP(s, "", true)
	applyIntegration(t, p, err)
	if !strings.Contains(read(t, path), "obituary") {
		t.Fatal("Qoder uninstall removed Pi's server")
	}
	p, err = planPluginMCP(s, path, "pi", "", true)
	applyIntegration(t, p, err)
	if strings.Contains(read(t, path), "obituary") {
		t.Fatal("last shared owner did not remove server")
	}
}

func TestQoderRejectsIndependentCNProjectHooks(t *testing.T) {
	s, dir := qoderTestScope(t, "qoder", false)
	path := filepath.Join(dir, "settings.json")
	input := `{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"C:/tools/obit.exe hook qoder-cn"}]}]}}`
	writeIntegrationFile(t, path, input)
	if _, err := PlanQoder(s, qoderTestCommands("qoder", "tools"), false); err == nil || !strings.Contains(err.Error(), "independent qoder-cn") {
		t.Fatal("duplicate variant installed", err)
	}
	if read(t, path) != input {
		t.Fatal("variant conflict changed original")
	}
}

func TestQoderRepeatedUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s, _ := qoderTestScope(t, "qoder", false)
	p, err := PlanQoder(s, qoderTestCommands("qoder", "tools"), false)
	applyIntegration(t, p, err)
	p, err = PlanQoderMCP(s, "C:/tools/obit.exe", false)
	applyIntegration(t, p, err)
	for range 2 {
		p, err = PlanQoder(s, Commands{}, true)
		applyIntegration(t, p, err)
		p, err = PlanQoderMCP(s, "", true)
		applyIntegration(t, p, err)
	}
}

func TestQoderSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "qoder")
}

func TestQoderSkillSharedProjectOwners(t *testing.T) {
	checkSharedSkillOwners(t, Scope{Project: t.TempDir()}, "qoder", "qoder-cn")
}

func TestQoderSkillUpgradeRetainsOwners(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	root := filepath.Join(s.Project, ".qoder", "skills")
	for _, name := range []string{"qoder", "qoder-cn"} {
		plan, err := planSkillAsset(s, root, "qoder-skill", name, []byte("previous skill"), false)
		applyIntegration(t, plan, err)
	}
	plan, err := PlanQoderSkill(s, false)
	applyIntegration(t, plan, err)
	plan, err = PlanQoderSkill(s, true)
	applyIntegration(t, plan, err)
	path := filepath.Join(root, "obituary", "SKILL.md")
	if read(t, path) != string(skillContent) {
		t.Fatal("shared upgrade lost payload or CN owner")
	}
	plan, err = PlanQoderCNSkill(s, true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("last owner retained upgraded skill", err)
	}
}
