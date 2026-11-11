package integrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/hook"
)

func TestCopilotMCPPreservesOtherOwners(t *testing.T) {
	t.Parallel()
	for _, owner := range []string{"claude", "codebuddy", "qoder", "minimax-code"} {
		t.Run(owner, func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			a, _ := LookupAgent(owner)
			plan, err := a.PlanMCP(s, `C:\tools\obit.exe`, false)
			applyIntegration(t, plan, err)
			path := filepath.Join(s.Project, ".mcp.json")
			original := read(t, path)
			plan, err = PlanCopilotMCP(s, `C:\tools\obit.exe`, true)
			applyIntegration(t, plan, err)
			if got := read(t, path); got != original {
				t.Fatalf("uninstalling never-installed Copilot changed %s MCP: %s", owner, got)
			}
		})
	}
}

// Shared native-host cases use actual temporary files and the public plan/apply
// boundary. Each host invokes them from its own paired test file.
type nativeJSONInstallFixture struct {
	agent                                        Agent
	scope                                        Scope
	dir, hookPath, rulesPath, mcpPath, skillPath string
	userConfig                                   string
}

func newNativeJSONInstallFixture(t *testing.T, name string, global bool) nativeJSONInstallFixture {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "repo")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COPILOT_HOME", filepath.Join(root, "copilot"))
	t.Setenv("GEMINI_CLI_HOME", filepath.Join(root, "gemini-home"))
	t.Setenv("FACTORY_HOME_OVERRIDE", filepath.Join(root, "droid-home"))
	t.Setenv("OBIT_USER_HOME", filepath.Join(root, "user"))
	a, ok := LookupAgent(name)
	if !ok || a.PlanMCP == nil || a.PlanSkill == nil {
		t.Fatal("missing planners", name)
	}
	if a.Capabilities.MCP.Level == hook.Unsupported || a.Capabilities.Skill.Level == hook.Unsupported {
		t.Fatal("installed extension capability missing")
	}
	f := nativeJSONInstallFixture{agent: a, scope: Scope{Global: global, Project: project, GuardRM: name == "copilot"}}
	f.dir = filepath.Join(project, "."+name)
	switch name {
	case "copilot":
		f.dir = filepath.Join(project, ".github")
		if global {
			f.dir = filepath.Join(root, "copilot")
		}
		f.hookPath = filepath.Join(f.dir, "hooks", "obituary.json")
		f.rulesPath = filepath.Join(f.dir, "copilot-instructions.md")
		f.mcpPath = filepath.Join(project, ".mcp.json")
		if global {
			f.mcpPath = filepath.Join(f.dir, "mcp-config.json")
		}
		f.userConfig = "\xef\xbb\xbf" + `{"theme":"dark","version":1,"hooks":{"agentStop":[{"type":"command","powershell":"user-tool"}]},"tail":true}`
	case "cursor":
		if global {
			f.dir = filepath.Join(root, "user", ".cursor")
		}
		f.hookPath = filepath.Join(f.dir, "hooks.json")
		f.rulesPath = filepath.Join(f.dir, "rules", "obituary.mdc")
		f.mcpPath = filepath.Join(f.dir, "mcp.json")
		f.userConfig = "\xef\xbb\xbf" + `{"theme":"dark","version":1,"hooks":{"stop":[{"command":"user-tool"}]},"tail":true}`
	case "gemini":
		if global {
			f.dir = filepath.Join(root, "gemini-home", ".gemini")
		}
		f.hookPath = filepath.Join(f.dir, "settings.json")
		f.mcpPath = f.hookPath
		f.rulesPath = filepath.Join(project, "GEMINI.md")
		if global {
			f.rulesPath = filepath.Join(f.dir, "GEMINI.md")
		}
		f.userConfig = "\xef\xbb\xbf{\n// keep this comment\n\"theme\":\"dark\",\"hooks\":{\"AfterAgent\":[{\"hooks\":[{\"type\":\"command\",\"command\":\"user-tool\"}]}]},\"tail\":true\n}\n"
	case "droid":
		f.dir = filepath.Join(project, ".factory")
		if global {
			f.dir = filepath.Join(root, "droid-home", ".factory")
		}
		f.hookPath = filepath.Join(f.dir, "hooks.json")
		f.rulesPath = filepath.Join(f.dir, "AGENTS.md")
		f.mcpPath = filepath.Join(f.dir, "mcp.json")
		f.userConfig = "\xef\xbb\xbf" + `{"Notification":[{"hooks":[{"type":"command","command":"user-tool"}]}],"Stop":[]}`
	}
	skillHome := project
	if global {
		skillHome = filepath.Join(root, "user")
		if name == "gemini" {
			skillHome = filepath.Join(root, "gemini-home")
		}
	}
	f.skillPath = filepath.Join(skillHome, ".agents", "skills", "obituary", "SKILL.md")
	return f
}

func nativeJSONCommands(name, location string) Commands {
	exe := "C:/Program Files/" + location + "/obit.exe"
	return Commands{Native: `"` + exe + `" hook ` + name, POSIX: "'/mnt/c/Program Files/" + location + "/obit.exe' hook " + name, Executable: exe}
}

func applyNativeJSONPlan(t *testing.T, f nativeJSONInstallFixture, changes []Change, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	err = Apply(changes)
	if f.agent.Name == "cursor" && f.scope.Global {
		if err == nil || !strings.Contains(err.Error(), "Cursor User Rules (UI)") {
			t.Fatalf("missing manual-rule outcome: %v", err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
}

func testNativeJSONInstallerLifecycle(t *testing.T, name string) {
	t.Helper()
	for _, global := range []bool{false, true} {
		label := "project"
		if global {
			label = "global"
		}
		t.Run(label, func(t *testing.T) {
			f := newNativeJSONInstallFixture(t, name, global)
			writeIntegrationFile(t, f.hookPath, f.userConfig)
			rulesBefore := "\xef\xbb\xbf# User rules\nKeep user prose.\n"
			if name != "cursor" {
				writeIntegrationFile(t, f.rulesPath, rulesBefore)
			} else {
				writeIntegrationFile(t, filepath.Join(f.dir, "rules", "user.mdc"), "user rule")
			}
			commands := nativeJSONCommands(name, "old")
			plan, err := f.agent.Plan(f.scope, commands, false)
			if err != nil {
				t.Fatal(err)
			}
			if read(t, f.hookPath) != f.userConfig {
				t.Fatal("dry-run wrote configuration")
			}
			applyNativeJSONPlan(t, f, plan, nil)
			installed := read(t, f.hookPath)
			if !strings.HasPrefix(installed, "\xef\xbb\xbf") || !strings.Contains(installed, "user-tool") || !strings.Contains(installed, "Program Files/old/obit.exe") {
				t.Fatal("lost user config/BOM/command", installed)
			}
			if name == "gemini" && !strings.Contains(installed, "// keep this comment") {
				t.Fatal("lost comment")
			}
			if name != "droid" && (strings.Index(installed, `"theme"`) > strings.Index(installed, `"hooks"`) || strings.Index(installed, `"hooks"`) > strings.Index(installed, `"tail"`)) {
				t.Fatal("user key order changed")
			}
			plan, err = f.agent.Plan(f.scope, commands, false)
			if err != nil || Pending(plan) {
				t.Fatalf("not idempotent: %+v %v", plan, err)
			}
			moved := nativeJSONCommands(name, "moved")
			plan, err = f.agent.Plan(f.scope, moved, false)
			applyNativeJSONPlan(t, f, plan, err)
			refreshed := read(t, f.hookPath)
			if strings.Contains(refreshed, "Program Files/old/") || !strings.Contains(refreshed, "Program Files/moved/") {
				t.Fatal("command not refreshed", refreshed)
			}
			plan, err = f.agent.Plan(f.scope, moved, true)
			applyNativeJSONPlan(t, f, plan, err)
			removed := read(t, f.hookPath)
			if strings.Contains(removed, "hook "+name) || !strings.Contains(removed, "user-tool") {
				t.Fatal("uninstall changed user handlers", removed)
			}
			if name != "cursor" && read(t, f.rulesPath) != rulesBefore {
				t.Fatal("user instructions changed")
			}
			if name == "cursor" {
				if _, err := os.Stat(f.rulesPath); !os.IsNotExist(err) {
					t.Fatal("managed project rule remains", err)
				}
				if read(t, filepath.Join(f.dir, "rules", "user.mdc")) != "user rule" {
					t.Fatal("user rule changed")
				}
			}
		})
	}
}

func testNativeJSONInstallerBoundaries(t *testing.T, name string) {
	t.Helper()
	for _, data := range []string{"", "\xef\xbb\xbf", "{", "[]"} {
		t.Run("invalid-"+data, func(t *testing.T) {
			f := newNativeJSONInstallFixture(t, name, false)
			writeIntegrationFile(t, f.hookPath, data)
			if _, err := f.agent.Plan(f.scope, nativeJSONCommands(name, "tools"), false); err == nil {
				t.Fatal("invalid config accepted")
			}
			if read(t, f.hookPath) != data {
				t.Fatal("invalid config changed")
			}
		})
	}
	t.Run("changed-after-plan", func(t *testing.T) {
		f := newNativeJSONInstallFixture(t, name, false)
		writeIntegrationFile(t, f.hookPath, f.userConfig)
		plan, err := f.agent.Plan(f.scope, nativeJSONCommands(name, "tools"), false)
		if err != nil {
			t.Fatal(err)
		}
		writeIntegrationFile(t, f.hookPath, "{\"savedByUser\":true}")
		if err := Apply(plan); err == nil {
			t.Fatal("stale plan applied")
		}
		if read(t, f.hookPath) != "{\"savedByUser\":true}" {
			t.Fatal("concurrent edit lost")
		}
		if _, err := os.Stat(f.rulesPath); !os.IsNotExist(err) {
			t.Fatal("preflight wrote an earlier asset", err)
		}
	})
	t.Run("outside-junction", func(t *testing.T) {
		f := newNativeJSONInstallFixture(t, name, false)
		outside := t.TempDir()
		makeIntegrationJunction(t, f.dir, outside)
		if _, err := f.agent.Plan(f.scope, nativeJSONCommands(name, "tools"), false); err == nil {
			t.Fatal("hook junction accepted")
		}
		if _, err := f.agent.PlanSkill(f.scope, false); err == nil {
			t.Fatal("skill junction accepted")
		}
		if name != "copilot" {
			if _, err := f.agent.PlanMCP(f.scope, "C:/tools/obit.exe", false); err == nil {
				t.Fatal("MCP junction accepted")
			}
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatal("outside directory changed", err)
		}
	})
}

func testNativeJSONExtensions(t *testing.T, name string) {
	t.Helper()
	for _, global := range []bool{false, true} {
		label := "project"
		if global {
			label = "global"
		}
		t.Run(label, func(t *testing.T) {
			f := newNativeJSONInstallFixture(t, name, global)
			original := "\xef\xbb\xbf" + `{"userSetting":7,"mcpServers":{"user":{"command":"user-tool","args":[]}},"tail":true}`
			writeIntegrationFile(t, f.mcpPath, original)
			plan, err := f.agent.PlanMCP(f.scope, "C:/old/obit.exe", false)
			if err != nil {
				t.Fatal(err)
			}
			if read(t, f.mcpPath) != original {
				t.Fatal("MCP dry-run wrote config")
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(read(t, f.mcpPath), "\xef\xbb\xbf") {
				t.Fatal("MCP lost BOM")
			}
			plan, err = f.agent.PlanMCP(f.scope, "C:/old/obit.exe", false)
			if err != nil || Pending(plan) {
				t.Fatal("MCP not idempotent", err)
			}
			var object map[string]any
			json.Unmarshal([]byte(strings.TrimPrefix(read(t, f.mcpPath), "\xef\xbb\xbf")), &object)
			entry := object["mcpServers"].(map[string]any)["obituary"].(map[string]any)
			if name == "copilot" {
				if entry["tools"] == nil {
					t.Fatal("Copilot tools not enabled")
				}
				entry["type"] = "local"
			}
			entry["env"] = map[string]any{"USER_VALUE": "kept"}
			entry["disabled"] = true
			entry["tools"] = []string{"obit_list"}
			data, _ := json.Marshal(object)
			writeIntegrationFile(t, f.mcpPath, string(data))
			plan, err = f.agent.PlanMCP(f.scope, "C:/moved/obit.exe", false)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			refreshed := read(t, f.mcpPath)
			for _, want := range []string{"C:/moved/obit.exe", "USER_VALUE", "kept", "obit_list", "user-tool"} {
				if !strings.Contains(refreshed, want) {
					t.Fatalf("lost %q: %s", want, refreshed)
				}
			}
			if err := json.Unmarshal([]byte(refreshed), &object); err != nil {
				t.Fatal(err)
			}
			if object["mcpServers"].(map[string]any)["obituary"].(map[string]any)["disabled"] != true {
				t.Fatal("disabled flag changed")
			}
			plan, err = f.agent.PlanMCP(f.scope, "C:/moved/obit.exe", true)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			if got := read(t, f.mcpPath); strings.Contains(got, `"obituary"`) || !strings.Contains(got, "user-tool") {
				t.Fatal("MCP uninstall lost users", got)
			}
			plan, err = f.agent.PlanSkill(f.scope, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(f.skillPath); !os.IsNotExist(err) {
				t.Fatal("skill dry-run wrote file")
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(read(t, f.skillPath)), skillContent) {
				t.Fatal("skill text was changed")
			}
			plan, err = f.agent.PlanSkill(f.scope, false)
			if err != nil || Pending(plan) {
				t.Fatal("skill not idempotent", err)
			}
			plan, err = f.agent.PlanSkill(f.scope, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(f.skillPath); !os.IsNotExist(err) {
				t.Fatal("skill not removed", err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(f.skillPath), ".obituary-skill")); !os.IsNotExist(err) {
				t.Fatal("skill receipt remains", err)
			}
		})
	}
	t.Run("edited-skill-preserved", func(t *testing.T) {
		f := newNativeJSONInstallFixture(t, name, false)
		plan, err := f.agent.PlanSkill(f.scope, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(plan); err != nil {
			t.Fatal(err)
		}
		writeIntegrationFile(t, f.skillPath, "user-modified skill")
		if _, err := f.agent.PlanSkill(f.scope, false); err == nil {
			t.Fatal("edited skill overwritten")
		}
		plan, err = f.agent.PlanSkill(f.scope, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(plan); err == nil {
			t.Fatal("preserved skill reported removed")
		}
		if read(t, f.skillPath) != "user-modified skill" {
			t.Fatal("edited skill deleted")
		}
	})
	t.Run("unowned-mcp-preserved", func(t *testing.T) {
		f := newNativeJSONInstallFixture(t, name, false)
		user := `{"mcpServers":{"obituary":{"command":"user-tool","args":["mcp"]}}}`
		writeIntegrationFile(t, f.mcpPath, user)
		if _, err := f.agent.PlanMCP(f.scope, "C:/tools/obit.exe", false); err == nil {
			t.Fatal("unowned MCP overwritten")
		}
		plan, err := f.agent.PlanMCP(f.scope, "C:/tools/obit.exe", true)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(plan); err == nil {
			t.Fatal("unowned MCP reported removed")
		}
		if read(t, f.mcpPath) != user {
			t.Fatal("user MCP changed")
		}
	})
}

func TestCopilotInstallerLifecycle(t *testing.T) { testNativeJSONInstallerLifecycle(t, "copilot") }
func TestCopilotInstallerBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testNativeJSONInstallerBoundaries(t, "copilot")
}
func TestCopilotInstallerExtensions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testNativeJSONExtensions(t, "copilot")
}

func TestCopilotNativeRegistrations(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "copilot", false)
	plan, err := PlanCopilot(f.scope, nativeJSONCommands("copilot", "tools"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	json.Unmarshal([]byte(read(t, f.hookPath)), &config)
	for _, event := range []string{"sessionStart", "postToolUse", "agentStop", "sessionEnd", "preToolUse"} {
		entries := config.Hooks[event]
		if len(entries) != 1 {
			t.Fatal("missing/duplicate hook", event)
		}
		entry := entries[0]
		ps, _ := entry["powershell"].(string)
		bash, _ := entry["bash"].(string)
		if !strings.HasPrefix(ps, "& ") || !strings.HasSuffix(ps, " --event "+event) || !strings.HasSuffix(bash, " --event "+event) || entry["timeoutSec"] != float64(5) {
			t.Fatalf("wrong native handler: %+v", entry)
		}
	}
	if config.Hooks["PreToolUse"] != nil {
		t.Fatal("compatible alias would duplicate calls")
	}
	f.scope.GuardRM = false
	plan, err = PlanCopilot(f.scope, nativeJSONCommands("copilot", "tools"), false)
	if err != nil || Pending(plan) {
		t.Fatal("ordinary refresh removed guard", err)
	}
}

func TestCopilotMCPSecondaryAndBareLayouts(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	f := newNativeJSONInstallFixture(t, "copilot", false)
	secondary := filepath.Join(f.dir, "mcp.json")
	seed, err := planSharedMCP(f.scope, secondary, "copilot", MCPConfig{ServersPath: []string{"mcpServers"}, Codec: copilotMCP{}}, "C:/old/obit.exe", false)
	applyIntegration(t, seed, err)
	writeIntegrationFile(t, secondary, `{"mcpServers":{"obituary":{"type":"local","command":"C:/old/obit.exe","args":["mcp"]},"user":{"command":"user-tool"}}}`)
	plan, err := PlanCopilotMCP(f.scope, "C:/moved/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, secondary), "C:/moved/obit.exe") {
		t.Fatal("shadowed old entry was not refreshed")
	}
	plan, err = PlanCopilotMCP(f.scope, "C:/moved/obit.exe", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, secondary), `"obituary"`) {
		t.Fatal("secondary definition would reactivate")
	}
	bare := `{"user":{"command":"user-tool","args":[]}}`
	writeIntegrationFile(t, f.mcpPath, bare)
	plan, err = PlanCopilotMCP(f.scope, "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	if read(t, f.mcpPath) != bare {
		t.Fatal("bare plan wrote the file")
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	installed := read(t, f.mcpPath)
	if strings.Contains(installed, `"mcpServers"`) || !strings.Contains(installed, "user-tool") || !strings.Contains(installed, "C:/tools/obit.exe") {
		t.Fatal("bare layout was not preserved", installed)
	}
	plan, err = PlanCopilotMCP(f.scope, "C:/tools/obit.exe", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	if got := read(t, f.mcpPath); strings.Contains(got, `"obituary"`) || !strings.Contains(got, "user-tool") {
		t.Fatal("bare uninstall changed users", got)
	}
}

func TestCopilotGlobalMCPRequiresItsDocumentedContainer(t *testing.T) {
	f := newNativeJSONInstallFixture(t, "copilot", true)
	before := `{"user":{"command":"user-tool","args":[]}}`
	writeIntegrationFile(t, f.mcpPath, before)
	if _, err := PlanCopilotMCP(f.scope, "C:/tools/obit.exe", false); err == nil || !strings.Contains(err.Error(), "mcpServers") {
		t.Fatal("invalid global layout accepted", err)
	}
	if read(t, f.mcpPath) != before {
		t.Fatal("global layout changed")
	}
}
