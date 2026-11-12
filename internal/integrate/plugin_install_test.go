package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginRulesPreserveNonStandaloneMarkers(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"pi", "omp", "opencode", "zcode", "minimax-code", "deepseek-harness"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			for _, suffix := range []string{" is an example", "\nKEEP THIS USER RULE\nexample "} {
				for _, uninstall := range []bool{false, true} {
					s := Scope{Project: t.TempDir()}
					path := filepath.Join(s.Project, "AGENTS.md")
					original := "User instructions\n<!-- obit:" + agent + ":begin -->" + suffix + "<!-- obit:" + agent + ":end -->\nOther instructions\n"
					writeIntegrationFile(t, path, original)
					plan, err := PlanAgent(agent, s, ownershipCommands(agent), uninstall)
					if err == nil {
						_ = Apply(plan)
						t.Errorf("non-standalone markers accepted (uninstall=%v)", uninstall)
					}
					if got := read(t, path); got != original {
						t.Fatalf("user rule example changed: %q", got)
					}
				}
			}
		})
	}
}

func TestPluginRulesMarkerLineEndings(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, eol := range []string{"\n", "\r\n"} {
		for _, prefix := range []string{"", "\xef\xbb\xbf", "\xef\xbb\xbfuser" + eol} {
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, "AGENTS.md")
			begin, end := "<!-- obit:pi:begin -->", "<!-- obit:pi:end -->"
			for _, block := range []string{begin + " example" + eol + "KEEP" + eol + end, begin + eol + "KEEP " + end, "example " + begin + eol + end, begin + eol + end + " suffix", begin + "\rKEEP" + eol + end, begin + eol + end + "\rKEEP"} {
				original := prefix + block + eol
				writeIntegrationFile(t, path, original)
				for _, uninstall := range []bool{false, true} {
					if _, err := planPluginRules(s, path, "pi", uninstall); err == nil {
						t.Fatalf("accepted invalid line boundary %q", original)
					}
					if read(t, path) != original {
						t.Fatal("malformed example changed")
					}
				}
			}
			writeIntegrationFile(t, path, prefix+begin+eol+"old"+eol+end+eol+"user tail"+eol)
			ch, err := planPluginRules(s, path, "pi", false)
			applyIntegration(t, []Change{ch}, err)
			got := read(t, path)
			if !strings.HasPrefix(got, prefix+begin+eol) || !strings.HasSuffix(got, end+eol+"user tail"+eol) {
				t.Fatalf("BOM or line endings changed: %q", got)
			}
			ch, err = planPluginRules(s, path, "pi", true)
			applyIntegration(t, []Change{ch}, err)
			if read(t, path) != prefix+eol+"user tail"+eol {
				t.Fatal("outside rule bytes changed")
			}
		}
	}
}

func pluginScope(t *testing.T, agent string, global bool) (Scope, string) {
	t.Helper()
	dir := t.TempDir()
	s := Scope{Project: dir, Global: global, GuardRM: true}
	if !global {
		return s, filepath.Join(dir, "."+agent)
	}
	switch agent {
	case "pi", "omp":
		t.Setenv("PI_CODING_AGENT_DIR", dir)
		t.Setenv("OMP_PROFILE", "")
		t.Setenv("PI_PROFILE", "")
	case "opencode":
		t.Setenv("OPENCODE_CONFIG_DIR", dir)
		t.Setenv("OPENCODE_CONFIG", "")
	case "openclaw":
		t.Setenv("OPENCLAW_STATE_DIR", dir)
		t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(dir, "openclaw.json"))
	}
	return s, dir
}

func testPluginLifecycle(t *testing.T, agent string) {
	t.Helper()
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "global"}[global], func(t *testing.T) {
			s, dir := pluginScope(t, agent, global)
			a, ok := LookupAgent(agent)
			if !ok {
				t.Fatalf("agent %s missing", agent)
			}
			cmd := Commands{Executable: testenv.NativeFixture(`C:\Program Files\obit\obit.exe`)}
			rules := filepath.Join(s.Project, "AGENTS.md")
			if global {
				rules = filepath.Join(dir, "AGENTS.md")
			}
			writeIntegrationFile(t, rules, "\xef\xbb\xbf# User\r\n<!-- keep -->\r\n")
			settings := filepath.Join(dir, "settings.json")
			writeIntegrationFile(t, settings, `{"theme":"dark","extensions":["user.ts"]}`)
			plan, err := a.Plan(s, cmd, false)
			if err != nil {
				t.Fatal(err)
			}
			if !Pending(plan) {
				t.Fatal("empty dry-run")
			}
			for _, c := range plan {
				if c.Action == "create" {
					if _, err := os.Stat(c.Path); !os.IsNotExist(err) {
						t.Fatalf("plan wrote %s", c.Path)
					}
				}
			}
			if err := Apply(plan); err != nil {
				t.Fatal(err)
			}
			if got := read(t, rules); !strings.HasPrefix(got, "\xef\xbb\xbf# User\r\n<!-- keep -->\r\n") {
				t.Fatalf("user rules changed: %q", got)
			}
			plan, err = a.Plan(s, cmd, false)
			if err != nil || Pending(plan) {
				t.Fatalf("not idempotent: %v %+v", err, plan)
			}
			cmd.Executable = testenv.NativeFixture(`C:\New Tools\obit.exe`)
			s.GuardRM = false
			plan, err = a.Plan(s, cmd, false)
			applyIntegration(t, plan, err)
			folder := "extensions"
			if agent == "opencode" {
				folder = "plugins"
			}
			runtime := filepath.Join(dir, folder, "obituary", agent+".json")
			var cfg map[string]any
			if err := json.Unmarshal([]byte(read(t, runtime)), &cfg); err != nil || cfg["executable"] != cmd.Executable || cfg["guardRM"] != true {
				t.Fatalf("refresh lost executable/guard: %v %v", cfg, err)
			}
			plan, err = a.Plan(s, cmd, true)
			applyIntegration(t, plan, err)
			if _, err := os.Stat(filepath.Join(dir, folder, "obituary.ts")); !os.IsNotExist(err) {
				t.Fatalf("plugin remains: %v", err)
			}
			if !strings.Contains(read(t, rules), "<!-- keep -->") || strings.Contains(read(t, rules), "obit:"+agent) {
				t.Fatal("rules uninstall")
			}
			if read(t, settings) != `{"theme":"dark","extensions":["user.ts"]}` {
				t.Fatal("settings changed")
			}
			plan, err = a.Plan(s, cmd, true)
			if err != nil || Pending(plan) {
				t.Fatalf("uninstall not idempotent: %v", err)
			}
			if _, err := a.PlanSkill(s, false); err != nil {
				t.Fatal("verified skill planning failed", err)
			}
		})
	}
}

func TestPluginPlansPreserveEditedAssetsAndRejectRaces(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"pi", "omp", "opencode"} {
		t.Run(agent, func(t *testing.T) {
			s, dir := pluginScope(t, agent, false)
			a, _ := LookupAgent(agent)
			cmd := Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}
			plan, err := a.Plan(s, cmd, false)
			applyIntegration(t, plan, err)
			folder := "extensions"
			if agent == "opencode" {
				folder = "plugins"
			}
			path := filepath.Join(dir, folder, "obituary.ts")
			original := read(t, path)
			writeIntegrationFile(t, path, original+"\n// user change")
			if _, err := a.Plan(s, cmd, false); err == nil {
				t.Fatal("overwrote edited plugin")
			}
			plan, err = a.Plan(s, cmd, true)
			if err != nil {
				t.Fatal(err)
			}
			if Apply(plan) == nil || !strings.HasSuffix(read(t, path), "// user change") {
				t.Fatal("edited plugin not preserved")
			}
		})
		t.Run(agent+"/race", func(t *testing.T) {
			s, _ := pluginScope(t, agent, false)
			a, _ := LookupAgent(agent)
			plan, err := a.Plan(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.Project, "AGENTS.md")
			writeIntegrationFile(t, path, "user saved")
			if Apply(plan) == nil {
				t.Fatal("accepted stale plan")
			}
			for _, c := range plan {
				if c.Path != path {
					if _, err := os.Stat(c.Path); !os.IsNotExist(err) {
						t.Fatalf("partial write %s", c.Path)
					}
				}
			}
		})
		t.Run(agent+"/junction", func(t *testing.T) {
			s, dir := pluginScope(t, agent, false)
			outside := t.TempDir()
			makeIntegrationJunction(t, dir, outside)
			a, _ := LookupAgent(agent)
			if _, err := a.Plan(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
				t.Fatal("followed external junction")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("outside target changed")
			}
		})
	}
}

func TestPluginRulesPreserveAndValidate(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	writeIntegrationFile(t, path, "user\r\n")
	plan, err := planPluginRules(s, path, "pi", false)
	applyIntegration(t, []Change{plan}, err)
	plan, err = planPluginRules(s, path, "omp", false)
	applyIntegration(t, []Change{plan}, err)
	plan, err = planPluginRules(s, path, "pi", true)
	applyIntegration(t, []Change{plan}, err)
	if !strings.Contains(read(t, path), "obit:omp:begin") {
		t.Fatal("removed other host rules")
	}
	writeIntegrationFile(t, path, "<!-- obit:pi:begin -->\nmissing end")
	if _, err := planPluginRules(s, path, "pi", false); err == nil {
		t.Fatal("accepted malformed block")
	}
}

func TestPluginPlansRejectUnownedRuntime(t *testing.T) {
	for _, agent := range []string{"pi", "omp", "opencode", "openclaw"} {
		for _, content := range []string{"", "\xef\xbb\xbf{}", "{broken", testenv.NativeFixture(`{\"executable\":\"C:/tools/obit.exe\",\"guardRM\":true}`)} {
			t.Run(agent+"/"+content, func(t *testing.T) {
				s, dir := pluginScope(t, agent, false)
				folder := "extensions"
				if agent == "opencode" {
					folder = "plugins"
				}
				path := filepath.Join(dir, folder, "obituary", agent+".json")
				if agent == "openclaw" {
					path = filepath.Join(dir, folder, "obituary", "runtime.json")
				}
				writeIntegrationFile(t, path, content)
				a, _ := LookupAgent(agent)
				if _, err := a.Plan(s, Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}, false); err == nil {
					t.Fatal("claimed unowned runtime")
				}
				if read(t, path) != content {
					t.Fatal("runtime changed")
				}
			})
		}
	}
}

func TestPluginRuntimeChangeAfterPlanRejected(t *testing.T) {
	s, dir := pluginScope(t, "pi", false)
	cmd := Commands{Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}
	plan, err := PlanPi(s, cmd, false)
	applyIntegration(t, plan, err)
	path := filepath.Join(dir, "extensions", "obituary", "pi.json")
	cmd.Executable = testenv.NativeFixture(`C:\new\obit.exe`)
	plan, err = PlanPi(s, cmd, false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, `{"saved":true}`)
	if Apply(plan) == nil || read(t, path) != `{"saved":true}` {
		t.Fatal("runtime stale plan replaced user file")
	}
}

func runPluginNodeTests(t *testing.T, host string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for plugin protocol verification:", err)
	}
	dir := t.TempDir()
	payload, err := pluginFiles.ReadFile("plugins/" + host + "/index.ts")
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := pluginFiles.ReadFile("plugins/bridge/transport.ts")
	if err != nil {
		t.Fatal(err)
	}
	payload = bytes.ReplaceAll(payload, []byte("/* OBIT_BRIDGE */"), bridge)
	module := filepath.Join(dir, "plugin.mjs")
	if err := os.WriteFile(module, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	// The test source imports the rendered installed module, with no host package
	// runtime or TypeScript compiler needed (the TS modules use erasable JS).
	cmd := exec.Command(node, "--test", filepath.Join("plugins", host, "index.test.mjs"))
	cmd.Env = append(os.Environ(), "OBIT_PLUGIN_MODULE="+module)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node plugin tests: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}
