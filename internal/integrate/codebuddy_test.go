package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func codeBuddyTestScope(t *testing.T, global bool) (Scope, string, string) {
	t.Helper()
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("CODEBUDDY_CONFIG_DIR", filepath.Join(home, ".codebuddy"))
	s := Scope{Global: global, Project: project}
	dir, rules := filepath.Join(project, ".codebuddy"), filepath.Join(project, "CODEBUDDY.md")
	if global {
		dir, rules = filepath.Join(home, ".codebuddy"), filepath.Join(home, ".codebuddy", "CODEBUDDY.md")
	}
	return s, dir, rules
}

func TestCodeBuddyInstallerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, name := range []string{"codebuddy", "codebuddy-ide"} {
		for _, global := range []bool{false, true} {
			t.Run(name+"/"+map[bool]string{true: "global", false: "project"}[global], func(t *testing.T) {
				s, dir, rules := codeBuddyTestScope(t, global)
				path := filepath.Join(dir, "settings.json")
				original := "\xef\xbb\xbf" + "{\n  // preserve comment\n  \"theme\":\"dark\",\n  \"hooks\":{\"PostToolUse\":[{\"matcher\":\"Write\",\"hooks\":[{\"type\":\"command\",\"command\":\"user-tool\"}]}]},\n  \"tail\":1,\n}\n"
				rulesBefore := "\xef\xbb\xbf# User rules\r\nKeep this text.\r\n"
				writeIntegrationFile(t, path, original)
				writeIntegrationFile(t, rules, rulesBefore)
				commands := Commands{Executable: testenv.NativeFixture("C:/Program Files/old/obit.exe")}
				plan, err := PlanAgent(name, s, commands, false)
				if err != nil || read(t, path) != original || read(t, rules) != rulesBefore {
					t.Fatal("dry-run", err)
				}
				applyIntegration(t, plan, nil)
				installed := read(t, path)
				if !strings.HasPrefix(installed, "\xef\xbb\xbf") || !strings.Contains(installed, "// preserve comment") || !strings.Contains(installed, "user-tool") || !strings.Contains(installed, "Program Files/old/obit.exe") || strings.Contains(installed, "PreToolUse") {
					t.Fatal("lost settings or installed unsupported guard", installed)
				}
				if strings.Index(installed, `"theme"`) > strings.Index(installed, `"hooks"`) || strings.Index(installed, `"hooks"`) > strings.Index(installed, `"tail"`) {
					t.Fatal("key order changed")
				}
				plan, err = PlanAgent(name, s, commands, false)
				if err != nil || Pending(plan) {
					t.Fatal("not idempotent", err, plan)
				}
				commands.Executable = testenv.NativeFixture("C:/Program Files/moved/obit.exe")
				plan, err = PlanAgent(name, s, commands, false)
				applyIntegration(t, plan, err)
				if got := read(t, path); strings.Contains(got, "Program Files/old/") || !strings.Contains(got, "Program Files/moved/") {
					t.Fatal("refresh", got)
				}
				plan, err = PlanAgent(name, s, commands, true)
				applyIntegration(t, plan, err)
				if got := read(t, path); strings.Contains(got, "hook codebuddy") || !strings.Contains(got, "user-tool") || !strings.Contains(got, "// preserve comment") {
					t.Fatal("uninstall", got)
				}
				if got := read(t, rules); got != rulesBefore {
					t.Fatalf("rules: %q", got)
				}
				plan, err = PlanAgent(name, s, commands, true)
				if err != nil || Pending(plan) {
					t.Fatal("repeated uninstall", err, plan)
				}
			})
		}
	}
}

func TestCodeBuddySharedHooksAndSkills(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s, dir, rules := codeBuddyTestScope(t, false)
	commands := Commands{Executable: testenv.NativeFixture("C:/Program Files/obit.exe")}
	for _, name := range []string{"codebuddy", "codebuddy-ide"} {
		plan, err := PlanAgent(name, s, commands, false)
		applyIntegration(t, plan, err)
		plan, err = PlanCodeBuddySharedSkill(s, name, false)
		applyIntegration(t, plan, err)
	}
	path := filepath.Join(dir, "settings.json")
	if strings.Count(read(t, path), "hook codebuddy") != 5 || strings.Contains(read(t, path), "hook codebuddy-ide") || strings.Count(read(t, rules), blockBegin) != 1 {
		t.Fatal("duplicate CLI/IDE registrations")
	}
	plan, err := PlanCodeBuddySharedSkill(s, "workbuddy", false)
	applyIntegration(t, plan, err)
	for _, name := range []string{"codebuddy", "codebuddy-ide"} {
		plan, err := PlanAgent(name, s, commands, true)
		applyIntegration(t, plan, err)
		plan, err = PlanCodeBuddySharedSkill(s, name, true)
		applyIntegration(t, plan, err)
		if name == "codebuddy" && (!strings.Contains(read(t, path), "hook codebuddy") || !strings.Contains(read(t, rules), blockBegin)) {
			t.Fatal("removed other hook owner")
		}
	}
	if strings.Contains(read(t, path), "hook codebuddy") {
		t.Fatal("last owner hook remains")
	}
	skill := filepath.Join(dir, "skills", "obituary", "SKILL.md")
	if read(t, skill) != string(skillContent) {
		t.Fatal("WorkBuddy skill removed")
	}
	plan, err = PlanCodeBuddySharedSkill(s, "workbuddy", true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatal("last skill owner remains", err)
	}
}

func TestCodeBuddyConfigurationScopes(t *testing.T) {
	s, dir, _ := codeBuddyTestScope(t, false)
	global := Scope{Global: true}
	commands := Commands{Executable: testenv.NativeFixture("C:/global/obit.exe")}
	plan, err := PlanCodeBuddyIDE(global, commands, false)
	applyIntegration(t, plan, err)
	if _, err := PlanCodeBuddy(s, commands, false); err == nil || !strings.Contains(err.Error(), "merges user and project") {
		t.Fatal("duplicated merged CLI hooks", err)
	}
	commands.Executable = testenv.NativeFixture("C:/project/obit.exe")
	plan, err = PlanCodeBuddyIDE(s, commands, false)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, filepath.Join(dir, "settings.json")), testenv.NativeFixture("C:/project/obit.exe")) {
		t.Fatal("IDE project target missing")
	}
	// This verifies installed layers; actual IDE precedence remains a host test.
	plan, err = PlanCodeBuddyIDE(s, commands, true)
	applyIntegration(t, plan, err)
	plan, err = PlanCodeBuddyIDE(global, Commands{Executable: testenv.NativeFixture("C:/global/obit.exe")}, false)
	if err != nil || Pending(plan) {
		t.Fatal("project uninstall changed user layer", err)
	}
}

func TestCodeBuddyInstallerBoundaries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, input := range []string{"", "\xef\xbb\xbf", "{", "[]", `{"hooks":null}`, `{"hooks":{"Stop":{}}}`} {
		t.Run("invalid-"+input, func(t *testing.T) {
			s, dir, _ := codeBuddyTestScope(t, false)
			path := filepath.Join(dir, "settings.json")
			writeIntegrationFile(t, path, input)
			if _, err := PlanCodeBuddy(s, Commands{Executable: testenv.NativeFixture("C:/obit.exe")}, false); err == nil {
				t.Fatal("invalid settings accepted")
			}
			if read(t, path) != input {
				t.Fatal("invalid config changed")
			}
		})
	}
	t.Run("changed-after-plan", func(t *testing.T) {
		s, dir, rules := codeBuddyTestScope(t, false)
		plan, err := PlanCodeBuddy(s, Commands{Executable: testenv.NativeFixture("C:/obit.exe")}, false)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "settings.json")
		writeIntegrationFile(t, path, `{"user":true}`)
		if Apply(plan) == nil || read(t, path) != `{"user":true}` {
			t.Fatal("concurrent save overwritten")
		}
		if _, err := os.Stat(rules); !os.IsNotExist(err) {
			t.Fatal("preflight wrote rules", err)
		}
	})
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "global-link", false: "project-link"}[global], func(t *testing.T) {
			s, dir, _ := codeBuddyTestScope(t, global)
			outside := t.TempDir()
			makeIntegrationJunction(t, dir, outside)
			if _, err := PlanCodeBuddy(s, Commands{Executable: testenv.NativeFixture("C:/obit.exe")}, false); err == nil {
				t.Fatal("link followed")
			}
			if _, err := PlanCodeBuddySharedSkill(s, "codebuddy", false); err == nil {
				t.Fatal("skill link followed")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("outside changed", err)
			}
		})
	}
}

func TestCodeBuddyDeleteGuardInstallation(t *testing.T) {
	for _, agent := range []string{"codebuddy", "codebuddy-ide"} {
		t.Run(agent, func(t *testing.T) {
			s, dir, _ := codeBuddyTestScope(t, false)
			s.GuardRM = true
			commands := Commands{Executable: testenv.NativeFixture("C:/obit.exe")}
			plan, err := PlanAgent(agent, s, commands, false)
			applyIntegration(t, plan, err)
			path := filepath.Join(dir, "settings.json")
			if !strings.Contains(read(t, path), "PreToolUse") {
				t.Fatal("guard hook missing")
			}
			s.GuardRM = false
			plan, err = PlanAgent(agent, s, commands, false)
			applyIntegration(t, plan, err)
			if !strings.Contains(read(t, path), "PreToolUse") {
				t.Fatal("refresh removed opted-in guard")
			}
			plan, err = PlanAgent(agent, s, commands, true)
			applyIntegration(t, plan, err)
			if strings.Contains(read(t, path), "hook codebuddy") {
				t.Fatal("uninstall left guard")
			}
		})
	}
}

func TestCodeBuddyMCPPriorityAndLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		for _, legacy := range []bool{false, true} {
			t.Run(map[bool]string{true: "global", false: "project"}[global]+map[bool]string{true: "/old", false: "/new"}[legacy], func(t *testing.T) {
				s, dir, _ := codeBuddyTestScope(t, global)
				root := s.Project
				if global {
					root = dir
				}
				file := ".mcp.json"
				if legacy {
					file = "mcp.json"
				}
				path := filepath.Join(root, file)
				original := "\xef\xbb\xbf{\n// existing services\n\"mcpServers\":{\"user\":{\"command\":\"user.exe\",\"args\":[],},},\n\"tail\":true,\n}\n"
				writeIntegrationFile(t, path, original)
				plan, err := PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/Program Files/old/obit.exe"), false)
				if err != nil || read(t, path) != original {
					t.Fatal("MCP dry-run", err)
				}
				applyIntegration(t, plan, err)
				if legacy {
					if _, err := os.Stat(filepath.Join(root, ".mcp.json")); !os.IsNotExist(err) {
						t.Fatal("legacy hidden", err)
					}
				}
				installed := read(t, path)
				if !strings.HasPrefix(installed, "\xef\xbb\xbf") || !strings.Contains(installed, "// existing services") || !strings.Contains(installed, "user.exe") {
					t.Fatal("lost user MCP", installed)
				}
				plan, err = PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/Program Files/old/obit.exe"), false)
				if err != nil || Pending(plan) {
					t.Fatal("MCP not idempotent", err)
				}
				plan, err = PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/Program Files/new/obit.exe"), false)
				applyIntegration(t, plan, err)
				if got := read(t, path); strings.Contains(got, "/old/") || !strings.Contains(got, "/new/") {
					t.Fatal("MCP refresh", got)
				}
				plan, err = PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/Program Files/new/obit.exe"), true)
				applyIntegration(t, plan, err)
				got, err := configJSON([]byte(read(t, path)), ConfigJSONC)
				want, _ := configJSON([]byte(original), ConfigJSONC)
				if err != nil || !sameConfigJSON(got, want) || !strings.Contains(read(t, path), "// existing services") {
					t.Fatal("MCP uninstall", err)
				}
				plan, err = PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/obit.exe"), true)
				if err != nil || Pending(plan) {
					t.Fatal("MCP repeated uninstall", err)
				}
			})
		}
	}
}

func TestCodeBuddyMCPManualAndSharedOwnership(t *testing.T) {
	s, dir, _ := codeBuddyTestScope(t, true)
	legacy := filepath.Join(filepath.Dir(dir), ".codebuddy.json")
	writeIntegrationFile(t, legacy, `{"projects":{},"mcpServers":{"user":{"command":"user.exe"}}}`)
	plan, err := PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/obit.exe"), false)
	if err != nil || PreservedError(plan) == nil || len(plan) != 1 || plan[0].Path != legacy {
		t.Fatal("legacy state silently edited", plan, err)
	}
	if _, err := PlanCodeBuddyIDEMCP(s, testenv.NativeFixture("C:/obit.exe"), false); err == nil || !strings.Contains(err.Error(), "MCP Server") {
		t.Fatal("guessed IDE MCP", err)
	}
	path := filepath.Join(dir, "selected-mcp.json")
	writeIntegrationFile(t, path, `{"mcpServers":{}}`)
	for _, owner := range []string{"codebuddy", "codebuddy-ide"} {
		plan, err := PlanCodeBuddyMCPFile(s, path, owner, testenv.NativeFixture("C:/obit.exe"), false)
		applyIntegration(t, plan, err)
	}
	plan, err = PlanCodeBuddyMCPFile(s, path, "codebuddy", testenv.NativeFixture("C:/obit.exe"), true)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), "obituary") {
		t.Fatal("removed other MCP owner")
	}
	plan, err = PlanCodeBuddyMCPFile(s, path, "codebuddy-ide", testenv.NativeFixture("C:/obit.exe"), true)
	applyIntegration(t, plan, err)
	if strings.Contains(read(t, path), "obituary") {
		t.Fatal("last MCP owner not removed")
	}
	writeIntegrationFile(t, path, testenv.NativeFixture(`{"mcpServers":{"obituary":{"command":"C:/other/obit.exe","args":["mcp"]}}}`))
	if _, err = PlanCodeBuddyMCPFile(s, path, "codebuddy", testenv.NativeFixture("C:/obit.exe"), false); err == nil {
		t.Fatal("claimed another host's MCP")
	}
	plan, err = PlanCodeBuddyMCPFile(s, path, "codebuddy", testenv.NativeFixture("C:/obit.exe"), true)
	if err != nil || PreservedError(plan) == nil {
		t.Fatal("unowned MCP silently removed", err)
	}
}

func TestCodeBuddyMCPRefusesChangedPriority(t *testing.T) {
	s, _, _ := codeBuddyTestScope(t, false)
	writeIntegrationFile(t, filepath.Join(s.Project, "mcp.json"), `{"mcpServers":{}}`)
	plan, err := PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, filepath.Join(s.Project, ".mcp.json"), `{"mcpServers":{"user":{}}}`)
	if Apply(plan) == nil {
		t.Fatal("changed file priority ignored")
	}
	if strings.Contains(read(t, filepath.Join(s.Project, "mcp.json")), "obituary") {
		t.Fatal("partial stale install")
	}
}

func TestCodeBuddyMCPKeepsUserFields(t *testing.T) {
	s, _, _ := codeBuddyTestScope(t, false)
	plan, err := PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/old/obit.exe"), false)
	applyIntegration(t, plan, err)
	path := filepath.Join(s.Project, ".mcp.json")
	raw, err := configJSON([]byte(read(t, path)), ConfigJSONC)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	entry := root["mcpServers"].(map[string]any)["obituary"].(map[string]any)
	entry["env"] = map[string]any{"CUSTOM": "keep"}
	entry["disabled"] = true
	data, _ := json.Marshal(root)
	writeIntegrationFile(t, path, string(data))
	plan, err = PlanCodeBuddyMCP(s, testenv.NativeFixture("C:/new/obit.exe"), false)
	applyIntegration(t, plan, err)
	if got := read(t, path); !strings.Contains(got, "CUSTOM") || !strings.Contains(got, `"disabled":true`) || !strings.Contains(got, testenv.NativeFixture("C:/new/obit.exe")) {
		t.Fatal("user server options lost", got)
	}
}

func TestCodeBuddyUnverifiedHookOwnershipIsReported(t *testing.T) {
	s, dir, rules := codeBuddyTestScope(t, false)
	path := filepath.Join(dir, "settings.json")
	content := testenv.NativeFixture(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"C:/old/obit.exe hook codebuddy"}]}]}}`)
	writeIntegrationFile(t, path, content)
	writeIntegrationFile(t, rules, blockBegin+"\nuser-edited instructions\n"+blockEnd+"\n")
	plan, err := PlanCodeBuddy(s, Commands{}, true)
	if err != nil || PreservedError(plan) == nil {
		t.Fatal("missing shared owner evidence reported as successful uninstall", err)
	}
	if read(t, path) != content {
		t.Fatal("unowned hook modified")
	}
}

func TestCodeBuddyRejectsSeparateIDEPostRegistration(t *testing.T) {
	s, dir, _ := codeBuddyTestScope(t, false)
	path := filepath.Join(dir, "settings.json")
	content := testenv.NativeFixture(`{"hooks":{"PostToolUse":[{"matcher":"write_to_file","hooks":[{"type":"command","command":"C:/old/obit.exe hook codebuddy-ide"}]}]}}`)
	writeIntegrationFile(t, path, content)
	if _, err := PlanCodeBuddy(s, Commands{Executable: testenv.NativeFixture("C:/obit.exe")}, false); err == nil {
		t.Fatal("separate IDE post hook would run twice")
	}
	if read(t, path) != content {
		t.Fatal("separate IDE hook modified")
	}
}
