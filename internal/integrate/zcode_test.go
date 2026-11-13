package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/agentid"
)

func zcodeScope(t *testing.T, global bool) (Scope, string, string) {
	t.Helper()
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	s := Scope{Global: global, Project: project}
	if global {
		return s, filepath.Join(home, ".zcode", "cli", "config.json"), filepath.Join(home, ".zcode", "AGENTS.md")
	}
	return s, filepath.Join(project, "zcode.json"), filepath.Join(project, "AGENTS.md")
}

func zcodePlanAll(s Scope, executable string, uninstall bool) ([]Change, error) {
	s.MCP = true
	plan, err := PlanZCode(s, Commands{Executable: executable}, uninstall)
	if err != nil {
		return nil, err
	}
	mcp, err := PlanZCodeMCP(s, executable, uninstall)
	if err != nil {
		return nil, err
	}
	return ComposeAgentChanges(append(plan, mcp...))
}

func zcodeReadMember(t *testing.T, path string, keys ...string) json.RawMessage {
	t.Helper()
	raw, err := configMember([]byte(read(t, path)), keys)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestZCodeInstallLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "user", false: "project"}[global], func(t *testing.T) {
			s, path, rules := zcodeScope(t, global)
			original := `{"first":"keep","hooks":{"enabled":false,"userFlag":true,"events":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"process","command":"user.exe","args":[]}]}]}},"features":{"mcp":false,"memory":false},"mcp":{"servers":{"user":{"type":"http","url":"https://example.invalid"}}},"last":"keep"}`
			writeIntegrationFile(t, path, original)
			userRules := "\xef\xbb\xbf# User rules\r\n<!-- user comment -->\r\n"
			writeIntegrationFile(t, rules, userRules)
			provider := filepath.Join(filepath.Dir(filepath.Dir(path)), "v2", "provider_config.json")
			writeIntegrationFile(t, provider, "provider sentinel")
			exe := testenv.NativeFixture(`C:\tools with spaces\中文 $quote'\obit.exe`)
			plan, err := zcodePlanAll(s, exe, false)
			if err != nil {
				t.Fatal(err)
			}
			if !global {
				var details string
				for _, ch := range plan {
					details += ch.Detail
				}
				if !strings.Contains(details, "--hook-digest") {
					t.Fatal("composed plan lost trust instructions")
				}
			}
			if read(t, path) != original || read(t, rules) != userRules {
				t.Fatal("Plan wrote files")
			}
			applyIntegration(t, plan, nil)
			installed := read(t, path)
			if strings.Index(installed, `"first"`) >= strings.Index(installed, `"last"`) || !strings.Contains(installed, "user.exe") {
				t.Fatal("user order/handler lost", installed)
			}
			for _, keys := range [][]string{{"hooks", "enabled"}, {"hooks", "userFlag"}, {"features", "mcp"}, {"mcp", "servers", "obituary", "enabled"}} {
				if !sameConfigJSON(zcodeReadMember(t, path, keys...), []byte("true")) {
					t.Fatal("gate missing", keys)
				}
			}
			if !sameConfigJSON(zcodeReadMember(t, path, "features", "memory"), []byte("false")) {
				t.Fatal("unrelated feature changed")
			}
			if zcodeReadMember(t, path, "hooks", "events", "SessionEnd") != nil || zcodeReadMember(t, path, "hooks", "events", "PreToolUse") != nil {
				t.Fatal("unverified events registered")
			}
			var groups []struct {
				Hooks []struct {
					Type, Command string
					Args          []string
					TimeoutMs     int
				}
			}
			if err := json.Unmarshal(zcodeReadMember(t, path, "hooks", "events", "SessionStart"), &groups); err != nil || len(groups) != 1 || len(groups[0].Hooks) != 1 {
				t.Fatalf("start handler: %+v %v", groups, err)
			}
			h := groups[0].Hooks[0]
			if h.Type != "process" || h.Command != exe || h.TimeoutMs != 5000 || !OwnedHookExec(h.Command, h.Args, agentid.ZCode, nil) {
				t.Fatalf("not fixed argv: %+v", h)
			}
			if !strings.HasPrefix(read(t, rules), userRules) || strings.Count(read(t, rules), "<!-- obit:zcode:begin -->") != 1 {
				t.Fatal("rules changed")
			}
			plan, err = zcodePlanAll(s, exe, false)
			if err != nil {
				t.Fatal(err)
			}
			for _, ch := range plan {
				if ch.Action != "unchanged" {
					t.Fatal("not idempotent", ch.Path, ch.Action)
				}
			}
			exe = testenv.NativeFixture(`C:\new tools\obit.exe`)
			plan, err = zcodePlanAll(s, exe, false)
			applyIntegration(t, plan, err)
			if err := json.Unmarshal(zcodeReadMember(t, path, "hooks", "events", "SessionStart"), &groups); err != nil || groups[0].Hooks[0].Command != exe {
				t.Fatal("hook executable stale", err)
			}
			if !sameConfigJSON(zcodeReadMember(t, path, "mcp", "servers", "obituary", "command"), mustJSON(exe)) {
				t.Fatal("MCP executable stale")
			}
			plan, err = zcodePlanAll(s, exe, true)
			applyIntegration(t, plan, err)
			if zcodeReadMember(t, path, "mcp", "servers", "obituary") != nil || zcodeReadMember(t, path, "hooks", "events", "SessionStart") != nil {
				t.Fatal("owned config survived uninstall")
			}
			if !strings.Contains(read(t, path), "user.exe") || zcodeReadMember(t, path, "mcp", "servers", "user") == nil || !sameConfigJSON(zcodeReadMember(t, path, "hooks", "enabled"), []byte("true")) || !sameConfigJSON(zcodeReadMember(t, path, "features", "mcp"), []byte("true")) {
				t.Fatal("uninstall touched user hooks/server/gates")
			}
			if strings.Contains(read(t, rules), "obit:zcode") || !strings.HasPrefix(read(t, rules), userRules) || read(t, provider) != "provider sentinel" {
				t.Fatal("user rules/provider damaged")
			}
			plan, err = zcodePlanAll(s, exe, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, ch := range plan {
				if ch.Action != "unchanged" {
					t.Fatal("uninstall not idempotent", ch.Path, ch.Action)
				}
			}
		})
	}
}

func TestZCodeMCPOnlyAndDisabledEntry(t *testing.T) {
	s, path, _ := zcodeScope(t, false)
	s.MCP = true
	writeIntegrationFile(t, path, testenv.NativeFixture(`{"hooks":{"enabled":false,"keep":true},"features":{"mcp":false},"mcp":{"servers":{"obituary":{"type":"stdio","command":"C:/old/obit.exe","args":["mcp"],"enabled":false,"env":{"KEY":"value"}}}}}`))
	plan, err := PlanZCodeMCP(s, testenv.NativeFixture(`C:\new\obit.exe`), false)
	applyIntegration(t, plan, err)
	for _, keys := range [][]string{{"hooks", "enabled"}, {"mcp", "servers", "obituary", "enabled"}} {
		if !sameConfigJSON(zcodeReadMember(t, path, keys...), []byte("false")) {
			t.Fatal("user disable flag lost", keys)
		}
	}
	if !sameConfigJSON(zcodeReadMember(t, path, "mcp", "servers", "obituary", "env"), []byte(`{"KEY":"value"}`)) {
		t.Fatal("env lost")
	}
	if !strings.Contains(plan[0].Detail, "enabled:false") {
		t.Fatal("disabled service reported without activation step")
	}
	gate, err := PlanConfigEdits(s, path, ConfigJSON, []ConfigEdit{{Path: []string{"features", "mcp"}, Value: json.RawMessage("false")}})
	applyIntegration(t, []Change{gate}, err)
	s.MCP = false
	plan, err = PlanZCodeMCP(s, testenv.NativeFixture(`C:\refreshed\obit.exe`), false)
	applyIntegration(t, plan, err)
	if !sameConfigJSON(zcodeReadMember(t, path, "features", "mcp"), []byte("false")) {
		t.Fatal("implicit refresh enabled MCP against user settings")
	}
	s.GuardRM = true
	if _, err := PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil || !strings.Contains(err.Error(), "CMD") {
		t.Fatal("unknown shell guard installation accepted", err)
	}
}

func TestZCodeConfigRejectionAndConcurrentEdit(t *testing.T) {
	for _, bad := range []string{"", " \r\n", "{broken", `[]`, `{"hooks":null}`, `{"hooks":{"enabled":"true"}}`, `{"features":{"mcp":null}}`, "\xef\xbb\xbf{}", "{ //comment\n}"} {
		t.Run(bad, func(t *testing.T) {
			s, path, _ := zcodeScope(t, false)
			writeIntegrationFile(t, path, bad)
			for _, mcp := range []bool{false, true} {
				var err error
				if mcp {
					_, err = PlanZCodeMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false)
				} else {
					_, err = PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false)
				}
				if err == nil || read(t, path) != bad {
					t.Fatalf("invalid config changed/accepted: %q %v", bad, err)
				}
			}
		})
	}
	s, path, rules := zcodeScope(t, false)
	writeIntegrationFile(t, path, `{"keep":1}`)
	plan, err := zcodePlanAll(s, testenv.NativeFixture(`C:\tools\obit.exe`), false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, `{"keep":2}`)
	if err := Apply(plan); err == nil {
		t.Fatal("stale plan accepted")
	}
	if read(t, path) != `{"keep":2}` {
		t.Fatal("concurrent edit overwritten")
	}
	if _, err := os.Stat(rules); !os.IsNotExist(err) {
		t.Fatal("partial rules write before preflight", err)
	}
}

func TestZCodeLayersAndLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	s, path, _ := zcodeScope(t, false)
	dot := filepath.Join(s.Project, ".zcode", "config.json")
	writeIntegrationFile(t, dot, `{"keep":1}`)
	plan, err := PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("shadow config created", err)
	}
	writeIntegrationFile(t, path, `{"keep":2}`)
	if _, err := PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
		t.Fatal("ambiguous project layers accepted")
	}
	plan, err = PlanZCode(s, Commands{}, true)
	applyIntegration(t, plan, err)
	if zcodeReadMember(t, dot, "hooks", "events") != nil || !sameConfigJSON(zcodeReadMember(t, path, "keep"), []byte("2")) {
		t.Fatal("layer uninstall failed")
	}
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "global-link", false: "project-link"}[global], func(t *testing.T) {
			s, path, _ := zcodeScope(t, global)
			link := filepath.Join(s.Project, ".zcode")
			if global {
				link = filepath.Dir(filepath.Dir(path))
			}
			outside := t.TempDir()
			makeIntegrationJunction(t, link, outside)
			if _, err := PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
				t.Fatal("external directory link accepted")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("external directory modified", err)
			}
		})
	}
}

func TestZCodeMCPConflictsAndHookOwnership(t *testing.T) {
	s, path, _ := zcodeScope(t, false)
	writeIntegrationFile(t, path, `{"mcp":{"servers":{"obituary":{"type":"http","url":"https://example.invalid"}}}}`)
	original := read(t, path)
	if _, err := PlanZCodeMCP(s, testenv.NativeFixture(`C:\tools\obit.exe`), false); err == nil {
		t.Fatal("unowned MCP overwritten")
	}
	plan, err := PlanZCodeMCP(s, "", true)
	if err != nil || len(plan) != 2 || plan[0].Action != "preserve" || plan[1].Action != "unchanged" {
		t.Fatal("unowned MCP removal plan", plan, err)
	}
	if err := Apply(plan); err == nil || read(t, path) != original {
		t.Fatal("unowned MCP was not preserved", err)
	}
	writeIntegrationFile(t, path, testenv.NativeFixture(`{"hooks":{"enabled":false,"events":{"SessionStart":[{"hooks":[{"type":"process","command":"C:/old/obit.exe","args":["hook","zcode","--help"]},{"type":"command","command":"obit hook zcode && user-task"}]}]}}}`))
	plan, err = PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false)
	applyIntegration(t, plan, err)
	plan, err = PlanZCode(s, Commands{}, true)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), "--help") || !strings.Contains(read(t, path), "user-task") {
		t.Fatal("unowned handlers removed")
	}
}

func TestZCodeProjectTrustInstructionsOnRefresh(t *testing.T) {
	s, _, _ := zcodeScope(t, false)
	s.Project = filepath.Join(s.Project, "project '$name`")
	path := filepath.Join(s.Project, "zcode.json")
	plan, err := PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\old\obit.exe`)}, false)
	applyIntegration(t, plan, err)
	declarations := zcodeReadMember(t, path, "hooks", "events")
	plan, err = PlanZCode(s, Commands{Executable: testenv.NativeFixture(`C:\new\obit.exe`)}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"trust review", "trust grant", "--hook-digest", "trust status", "create a new session"} {
		if !strings.Contains(plan[0].Detail, want) {
			t.Fatal("missing trust step", want, plan[0].Detail)
		}
	}
	quoted := "'" + strings.ReplaceAll(s.Project, "'", "'\"'\"'") + "'"
	words, ok := shellWords("zcode hooks trust review --workspace " + quoted + " --json")
	if !ok || len(words) != 7 || words[5] != s.Project || !strings.Contains(plan[0].Detail, "--workspace "+quoted) {
		t.Fatal("workspace quoting changed manual command arguments", words)
	}
	applyIntegration(t, plan, nil)
	if bytes.Equal(declarations, zcodeReadMember(t, path, "hooks", "events")) {
		t.Fatal("changed executable did not change reviewed declarations")
	}
	for _, ch := range plan {
		if ch.Path != path && filepath.Base(ch.Path) != "AGENTS.md" {
			t.Fatal("unexpected trust/provider write", ch.Path)
		}
	}
}

func TestZCodeSkillLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "user", false: "project"}[global], func(t *testing.T) {
			s, config, rules := zcodeScope(t, global)
			root := filepath.Join(s.Project, ".zcode")
			if global {
				root = filepath.Dir(rules)
			}
			path := filepath.Join(root, "skills", "obituary", "SKILL.md")
			plan, err := PlanZCodeSkill(s, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("skill Plan wrote", err)
			}
			applyIntegration(t, plan, nil)
			if read(t, path) != string(skillContent) {
				t.Fatal("skill frontmatter/content changed")
			}
			if _, err := os.Stat(config); !os.IsNotExist(err) {
				t.Fatal("skill planner enabled host features", err)
			}
			plan, err = PlanZCodeSkill(s, false)
			if err != nil {
				t.Fatal(err)
			}
			for _, ch := range plan {
				if ch.Action != "unchanged" {
					t.Fatal("skill not idempotent", ch.Action)
				}
			}
			plan, err = PlanZCodeSkill(s, true)
			applyIntegration(t, plan, err)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("owned skill not removed", err)
			}
			plan, err = PlanZCodeSkill(s, false)
			applyIntegration(t, plan, err)
			writeIntegrationFile(t, path, "user changed skill")
			if _, err := PlanZCodeSkill(s, false); err == nil {
				t.Fatal("user skill overwritten")
			}
			plan, err = PlanZCodeSkill(s, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err == nil || read(t, path) != "user changed skill" {
				t.Fatal("user skill not preserved", err)
			}
		})
	}
}
