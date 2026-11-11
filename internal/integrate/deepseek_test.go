package integrate

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"go.yaml.in/yaml/v3"
)

func deepseekScope(t *testing.T, global bool) (Scope, string) {
	t.Helper()
	dir := t.TempDir()
	s := Scope{Project: dir, Global: global, GuardRM: true}
	if global {
		t.Setenv("DSH_HOME", filepath.Join(dir, "dsh-home"))
	}
	root, err := deepseekRoot(s)
	if err != nil {
		t.Fatal(err)
	}
	return s, root
}

func deepseekCommands(path string) Commands {
	return Commands{Executable: path, Native: `"` + path + `" hook deepseek-harness`}
}

func TestDeepSeekBundleLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "home", false: "project"}[global], func(t *testing.T) {
			s, root := deepseekScope(t, global)
			config := filepath.Join(root, "cordis.patch.yml")
			original := "\xef\xbb\xbf# provider stays first\n- id: llm\n  config:\n    key: !!js process.env.API_KEY\n    model: user-model\n# user tail\n"
			writeIntegrationFile(t, config, original)
			rules := filepath.Join(s.Project, "AGENTS.md")
			if global {
				rules = filepath.Join(root, "AGENTS.md")
			}
			writeIntegrationFile(t, rules, "\xef\xbb\xbf# User\r\n<!-- user comment -->\r\n")
			commands := deepseekCommands(`C:\Program Files\讣告\obit.exe`)
			plan, err := PlanDeepSeek(s, commands, false)
			if err != nil {
				t.Fatal(err)
			}
			if read(t, config) != original {
				t.Fatal("dry run changed provider")
			}
			for _, ch := range plan {
				if ch.Action == "create" {
					if _, err := os.Stat(ch.Path); !os.IsNotExist(err) {
						t.Fatal("plan wrote")
					}
				}
			}
			applyIntegration(t, plan, err)
			dir := filepath.Join(root, "obituary-hooks")
			var patch []struct {
				Insert []struct {
					ID, Name string
					Config   map[string]any
				}
			}
			if err := yaml.Unmarshal([]byte(read(t, filepath.Join(dir, "cordis.patch.yml"))), &patch); err != nil {
				t.Fatal(err)
			}
			if len(patch) != 1 || len(patch[0].Insert) != 2 || patch[0].Insert[0].Name != "@deepseek-ai/dsh-hooks-claude-code" || patch[0].Insert[0].Config["configPath"] != filepath.Join(dir, "hooks.json") || patch[0].Insert[0].Config["defaultTimeoutMs"] != 5000 {
				t.Fatalf("invalid overlay: %+v", patch)
			}
			if strings.Contains(read(t, filepath.Join(dir, "hooks.json")), "SessionEnd") {
				t.Fatal("invented SessionEnd")
			}
			if !strings.Contains(read(t, filepath.Join(dir, "hooks.json")), `^(bash|pwsh)$`) {
				t.Fatal("guard matcher missing")
			}
			if !strings.Contains(read(t, filepath.Join(dir, "index.ts")), "processPathFromHostPath") {
				t.Fatal("observer missing locality proof")
			}
			plan, err = PlanDeepSeek(s, commands, false)
			if err != nil || Pending(plan) {
				t.Fatalf("not idempotent: %v", err)
			}
			s.GuardRM = false
			commands = deepseekCommands(`C:\New Tools\obit.exe`)
			plan, err = PlanDeepSeek(s, commands, false)
			applyIntegration(t, plan, err)
			if got := read(t, filepath.Join(dir, "hooks.json")); !strings.Contains(got, "New Tools") || !strings.Contains(got, "PreToolUse") {
				t.Fatalf("refresh lost guard/path: %s", got)
			}
			plan, err = PlanDeepSeek(s, Commands{}, true)
			applyIntegration(t, plan, err)
			for _, file := range []string{"hooks.json", "index.ts", "cordis.patch.yml", "package.json"} {
				if _, err := os.Stat(filepath.Join(dir, file)); !os.IsNotExist(err) {
					t.Fatalf("asset remains %s %v", file, err)
				}
			}
			if read(t, config) != original || !strings.Contains(read(t, rules), "<!-- user comment -->") || strings.Contains(read(t, rules), "obit:deepseek-harness") {
				t.Fatal("uninstall touched user data")
			}
			plan, err = PlanDeepSeek(s, Commands{}, true)
			if err != nil || Pending(plan) {
				t.Fatalf("uninstall not idempotent: %v", err)
			}
		})
	}
}

func TestDeepSeekProfilesAndHomeLayersRemainIndependent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s, root := deepseekScope(t, true)
	commands := deepseekCommands(`C:\tools\obit.exe`)
	paths := []string{filepath.Join(root, "cordis.patch.yml"), filepath.Join(root, "profiles", "web", "cordis.patch.yml"), filepath.Join(root, "profiles", "headless", "cordis.patch.yml")}
	originals := []string{"# home\n- id: provider\n  config:\n    token: !!js process.env.TOKEN\n", "# web\n- id: provider\n  config: {model: web}\n", "[malformed user YAML"}
	for i, p := range paths {
		writeIntegrationFile(t, p, originals[i])
	}
	for _, profile := range []string{"web", "headless"} {
		plan, err := PlanDeepSeekProfile(s, commands, profile, false)
		applyIntegration(t, plan, err)
		plan, err = PlanDeepSeekProfileMCP(s, profile, commands.Executable, false)
		applyIntegration(t, plan, err)
	}
	plan, err := PlanDeepSeek(s, commands, false)
	applyIntegration(t, plan, err)
	plan, err = PlanDeepSeekProfile(s, Commands{}, "web", true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(filepath.Join(root, "profiles", "headless", "obituary-hooks", "hooks.json")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, "AGENTS.md")), "obit:deepseek-harness") {
		t.Fatal("profile removed home instructions")
	}
	for i, p := range paths {
		if read(t, p) != originals[i] {
			t.Fatalf("rewrote layer %s", p)
		}
	}
	// Complete owned rows include every required key, so later whole-config
	// replacement never relies on merging with a profile/provider row.
	patch := read(t, filepath.Join(root, "obituary-hooks", "cordis.patch.yml"))
	for _, required := range []string{"id: obituary-deepseek-hooks", "configPath:", "defaultTimeoutMs:", "id: obituary-deepseek-writes", "executable:"} {
		if !strings.Contains(patch, required) {
			t.Fatalf("incomplete row %s", required)
		}
	}
	if strings.Contains(patch, "id: provider") {
		t.Fatal("overrode provider")
	}
	if _, err := PlanDeepSeekProfile(Scope{Project: t.TempDir()}, commands, "web", false); err == nil {
		t.Fatal("accepted project as profile")
	}
	if _, err := userdirs.DeepSeekProfileDir("../web"); err == nil {
		t.Fatal("accepted escaping profile")
	}
}

func TestDeepSeekMCPLifecycle(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "home", false: "project"}[global], func(t *testing.T) {
			s, root := deepseekScope(t, global)
			path := filepath.Join(root, "obituary-mcp", "cordis.patch.yml")
			plan, err := PlanDeepSeekMCP(s, `C:\Program Files\obit.exe`, false)
			applyIntegration(t, plan, err)
			var patch []struct {
				Insert []struct {
					Name   string
					Config struct {
						ServerName, Transport, Command string
						Args                           []string
					}
				}
			}
			if err := yaml.Unmarshal([]byte(read(t, path)), &patch); err != nil {
				t.Fatal(err)
			}
			// JSON is the YAML 1.2 subset; inspect JSON member spellings too.
			var raw []map[string][]map[string]any
			if err := json.Unmarshal([]byte(read(t, path)), &raw); err != nil {
				t.Fatal(err)
			}
			row := raw[0]["insert"][0]
			cfg := row["config"].(map[string]any)
			if row["name"] != "@deepseek-ai/dsh-mcp-client" || cfg["serverName"] != "obituary" || cfg["transport"] != "stdio" || cfg["command"] != `C:\Program Files\obit.exe` {
				t.Fatalf("%+v", row)
			}
			plan, err = PlanDeepSeekMCP(s, `C:\Program Files\obit.exe`, false)
			if err != nil || Pending(plan) {
				t.Fatalf("%v", err)
			}
			plan, err = PlanDeepSeekMCP(s, `C:\new\obit.exe`, false)
			applyIntegration(t, plan, err)
			if !strings.Contains(read(t, path), "new") {
				t.Fatal("stale executable")
			}
			plan, err = PlanDeepSeekMCP(s, "", true)
			applyIntegration(t, plan, err)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("MCP remains")
			}
			for _, ch := range plan {
				if strings.Contains(ch.Path, ".claude") || strings.Contains(ch.Path, ".codex") {
					t.Fatal("wrote another host")
				}
			}
		})
	}
}

func TestDeepSeekPreservesUnownedAndEditedAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, file := range []string{"hooks.json", "index.ts", "cordis.patch.yml", "package.json"} {
		for _, content := range []string{"", "\xef\xbb\xbf{}", "{bad", "# user\n!!js process.env.API_KEY"} {
			t.Run(file+"/"+content, func(t *testing.T) {
				s, root := deepseekScope(t, false)
				path := filepath.Join(root, "obituary-hooks", file)
				writeIntegrationFile(t, path, content)
				if _, err := PlanDeepSeek(s, deepseekCommands(`C:\tools\obit.exe`), false); err == nil {
					t.Fatal("claimed unowned asset")
				}
				plan, err := PlanDeepSeek(s, Commands{}, true)
				if err != nil {
					t.Fatal(err)
				}
				if Apply(plan) == nil || read(t, path) != content {
					t.Fatal("lost unowned asset")
				}
			})
		}
	}
	s, root := deepseekScope(t, false)
	cmd := deepseekCommands(`C:\tools\obit.exe`)
	plan, err := PlanDeepSeek(s, cmd, false)
	applyIntegration(t, plan, err)
	path := filepath.Join(root, "obituary-hooks", "hooks.json")
	original := read(t, path) + "\n "
	writeIntegrationFile(t, path, original)
	if _, err := PlanDeepSeek(s, cmd, false); err == nil {
		t.Fatal("overwrote user edit")
	}
	plan, err = PlanDeepSeek(s, Commands{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if Apply(plan) == nil || read(t, path) != original {
		t.Fatal("edited asset removed")
	}
}

func TestDeepSeekPlansRejectRacesAndLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "home", false: "project"}[global], func(t *testing.T) {
			s, root := deepseekScope(t, global)
			cmd := deepseekCommands(`C:\tools\obit.exe`)
			plan, err := PlanDeepSeek(s, cmd, false)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "obituary-hooks", "hooks.json")
			writeIntegrationFile(t, path, "user saved")
			if Apply(plan) == nil || read(t, path) != "user saved" {
				t.Fatal("accepted stale plan")
			}
			if _, err := os.Stat(filepath.Join(root, "obituary-hooks", "package.json")); !os.IsNotExist(err) {
				t.Fatal("partial install")
			}
		})
	}
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "global-link", false: "project-link"}[global], func(t *testing.T) {
			s, root := deepseekScope(t, global)
			outside := t.TempDir()
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			makeIntegrationJunction(t, filepath.Join(root, "obituary-hooks"), outside)
			if _, err := PlanDeepSeek(s, deepseekCommands(`C:\tools\obit.exe`), false); err == nil {
				t.Fatal("followed link")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("changed external directory")
			}
		})
	}
}

func TestDeepSeekSkillAndCapabilities(t *testing.T) {
	for _, global := range []bool{false, true} {
		s, root := deepseekScope(t, global)
		plan, err := PlanDeepSeekSkill(s, false)
		applyIntegration(t, plan, err)
		path := filepath.Join(root, "skills", "obituary", "SKILL.md")
		if read(t, path) != string(skillContent) {
			t.Fatal("skill content drift")
		}
		plan, err = PlanDeepSeekSkill(s, false)
		if err != nil || Pending(plan) {
			t.Fatal("skill not idempotent", err)
		}
		plan, err = PlanDeepSeekSkill(s, true)
		applyIntegration(t, plan, err)
	}
	agent, ok := LookupAgent("deepseek-harness")
	if !ok || agent.Capabilities.DeleteGuard.Level != hook.Limited || agent.Capabilities.SessionStart.Level != hook.Limited || agent.Capabilities.SessionEnd.Level != hook.Unsupported || agent.Capabilities.MCP.Level != hook.Limited {
		t.Fatalf("capabilities: %+v", agent)
	}
}

func TestDeepSeekObserverProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := deepseekFiles.ReadFile("plugins/deepseek/index.ts")
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := pluginFiles.ReadFile("plugins/bridge/transport.ts")
	if err != nil {
		t.Fatal(err)
	}
	payload = bytes.ReplaceAll(payload, []byte("/* OBIT_BRIDGE */"), bridge)
	path := filepath.Join(t.TempDir(), "observer.mjs")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "--test", filepath.Join("plugins", "deepseek", "index.test.mjs"))
	cmd.Env = append(os.Environ(), "OBIT_PLUGIN_MODULE="+path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}
