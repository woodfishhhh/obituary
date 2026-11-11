package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func TestDoctorAgentShellDeclaration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"codebuddy", "codebuddy-ide", "minimax-code"} {
		t.Run(agent, func(t *testing.T) {
			root, executable := doctorAgentFixture(t)
			path, err := config.Path()
			if err != nil {
				t.Fatal(err)
			}
			for _, shell := range []string{"", "bash", "powershell"} {
				data := "# owner config\n"
				if shell != "" {
					data += "[agent_shells]\n" + agent + " = '" + shell + "'\n"
				}
				clitest.WriteFixture(t, path, data)
				before := clitest.TreeSnapshot(t, root)
				got := diagnoseIntegration(agent, initcmd.NewAgentInspector(integrate.Scope{Global: true}, executable))
				want := "agent_shells." + agent + " = \"" + shell + "\""
				if shell == "" {
					want = "agent_shells." + agent + " is unset"
				}
				if got.Status == "FAIL" || !strings.Contains(got.Detail, want) {
					t.Fatalf("declaration diagnosis: %+v", got)
				}
				if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
					t.Fatal("doctor modified declaration or host settings")
				}
			}
		})
	}
}

func TestDoctorSummarizesUninstalledAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("checks the complete registered-agent inventory")
	}
	root, executable := doctorAgentFixture(t)
	// An agent's own configuration and another tool's hook are not Obituary installations.
	clitest.WriteFixture(t, filepath.Join(root, "home", ".gemini", "settings.json"), `{"hooks":{"AfterTool":[{"hooks":[{"type":"command","command":"other-tool hook gemini"}]}]}}`)
	clitest.WriteFixture(t, filepath.Join(root, "home", ".cursor", "hooks.json"), "{")
	clitest.WriteFixture(t, filepath.Join(root, "opencode", "plugins", "obituary.ts"), "user plugin without a managed receipt")
	before := clitest.TreeSnapshot(t, root)
	checks, err := doctorChecks(t, executable, initcmd.AgentNames()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range initcmd.AgentNames() {
		check, exists := checks[name]
		if name == "claude" || name == "codex" {
			if !exists || check.Status != "WARN" || check.Installation != "missing" {
				t.Fatalf("historical agent check: %+v", check)
			}
		} else if exists {
			t.Fatalf("uninstalled agent was diagnosed: %+v", check)
		}
	}
	for _, check := range checks {
		if check.Capabilities != nil {
			t.Fatalf("default JSON includes capabilities: %+v", check)
		}
		if check.Status == "WARN" {
			switch check.Name {
			case "runner", "tray-startup", "gui", "start-menu", "desktop", "claude", "codex", "codex-windows-shell":
			default:
				t.Fatalf("fresh doctor added a warning: %+v", check)
			}
		}
	}
	want := fmt.Sprintf("另有 %d 家 agent 可接入：obit init --show 查看", len(initcmd.AgentNames())-2)
	rows := diagnose(time.Now(), executable)
	last := rows[len(rows)-1]
	if len(rows) != 33 || last.Status != "INFO" || last.Detail != want {
		t.Fatalf("inventory summary: %+v", rows)
	}
	if check := checks["registration-identities"]; check.Status != "PASS" || check.Errors != 0 {
		t.Fatalf("fresh identity diagnostic: %+v", check)
	}
	out, _, err := clitest.Execute(t, NewCmdWithExecutable(executable))
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out), want) || strings.Contains(out, "delete guard:") || strings.Contains(out, "实际验证:") {
		t.Fatalf("default text is noisy: %s %v", out, err)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("doctor changed installation files")
	}
	t.Logf("fresh doctor: %d checks, %d bytes, %d extra agents summarized", len(rows), len(out), len(initcmd.AgentNames())-2)
}

func TestDoctorExtraAgentLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("checks inventory counts across the full agent lifecycle")
	}
	root, executable := doctorAgentFixture(t)
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "gemini", "--yes")
	if err != nil {
		t.Fatal(out, err)
	}
	check := func(status, installation string) {
		t.Helper()
		before := clitest.TreeSnapshot(t, root)
		checks, err := doctorChecks(t, executable, initcmd.AgentNames()...)
		if err != nil || checks["gemini"].Status != status || checks["gemini"].Installation != installation {
			t.Fatalf("gemini: %+v %v", checks["gemini"], err)
		}
		want := fmt.Sprintf("另有 %d 家 agent 可接入：obit init --show 查看", len(initcmd.AgentNames())-3)
		if checks["agents"].Detail != want {
			t.Fatalf("installed agent counted as available: %+v", checks["agents"])
		}
		if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
			t.Fatal("doctor changed installation files")
		}
	}
	check("PASS", "installed")
	path := filepath.Join(root, "home", ".gemini", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// init writes the absolute executable; an outdated installation points elsewhere.
	self, err := executable.Path()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), filepath.ToSlash(self)) {
		t.Fatalf("installed hook does not name %s: %s", self, data)
	}
	clitest.WriteFixture(t, path, strings.ReplaceAll(string(data), filepath.ToSlash(self), "C:/old/obit.exe"))
	check("WARN", "outdated")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	check("WARN", "outdated")
	out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "gemini", "--uninstall", "--yes")
	if err != nil {
		t.Fatal(out, err)
	}
	checks, err := doctorChecks(t, executable, initcmd.AgentNames()...)
	if _, exists := checks["gemini"]; err != nil || exists {
		t.Fatalf("uninstalled gemini remains: %+v %v", checks["gemini"], err)
	}
}

func TestDoctorFindsExtraAgentInstallationTraces(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tc := range []struct {
		name, agent string
		install     func(*testing.T, string, cmdutil.Executable)
		status      string
	}{
		{"hook entry", "gemini", func(t *testing.T, root string, executable cmdutil.Executable) {
			clitest.WriteFixture(t, filepath.Join(root, "home", ".gemini", "settings.json"), `{"hooks":{"AfterTool":[{"hooks":[{"type":"command","command":"C:/old/obit.exe hook gemini --event AfterTool"}]}]}}`)
		}, "WARN"},
		{"rule marker", "gemini", func(t *testing.T, root string, executable cmdutil.Executable) {
			clitest.WriteFixture(t, filepath.Join(root, "home", ".gemini", "GEMINI.md"), "<!-- obit:begin (managed by `obit init`; edits here are replaced) -->\nold instructions\n<!-- obit:end -->\n")
		}, "WARN"},
		{"MCP entry", "gemini", func(t *testing.T, root string, executable cmdutil.Executable) {
			clitest.WriteFixture(t, filepath.Join(root, "home", ".gemini", "settings.json"), `{"mcpServers":{"obituary":{"command":"obit","args":["mcp"]}}}`)
		}, "WARN"},
		{"skill receipt", "gemini", func(t *testing.T, root string, executable cmdutil.Executable) {
			changes, err := integrate.PlanGeminiSkill(integrate.Scope{Global: true}, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := integrate.Apply(changes); err != nil {
				t.Fatal(err)
			}
		}, "WARN"},
		{"missing skill with receipt", "gemini", func(t *testing.T, root string, executable cmdutil.Executable) {
			changes, err := integrate.PlanGeminiSkill(integrate.Scope{Global: true}, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := integrate.Apply(changes); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(root, "home", ".gemini", "skills", "obituary", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		}, "WARN"},
		{"managed receipt", "opencode", func(t *testing.T, root string, executable cmdutil.Executable) {
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "opencode", "--yes")
			if err != nil {
				t.Fatal(out, err)
			}
		}, "PASS"},
		{"edited managed asset", "opencode", func(t *testing.T, root string, executable cmdutil.Executable) {
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "opencode", "--yes")
			if err != nil {
				t.Fatal(out, err)
			}
			clitest.WriteFixture(t, filepath.Join(root, "opencode", "plugins", "obituary.ts"), "edited plugin")
			if err := os.Remove(filepath.Join(root, "opencode", "AGENTS.md")); err != nil {
				t.Fatal(err)
			}
		}, "FAIL"},
		{"missing managed asset", "opencode", func(t *testing.T, root string, executable cmdutil.Executable) {
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "opencode", "--yes")
			if err != nil {
				t.Fatal(out, err)
			}
			for _, path := range []string{filepath.Join(root, "opencode", "plugins", "obituary.ts"), filepath.Join(root, "opencode", "AGENTS.md")} {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
		}, "WARN"},
		{"manual activation", "openclaw", func(t *testing.T, root string, executable cmdutil.Executable) {
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "openclaw", "--yes")
			if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(out, "需要手动步骤") {
				t.Fatal(out, err)
			}
		}, "WARN"},
		{"manual rules", "cursor", func(t *testing.T, root string, executable cmdutil.Executable) {
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "cursor", "--yes")
			if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(out, "需要手动步骤") {
				t.Fatal(out, err)
			}
		}, "WARN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, executable := doctorAgentFixture(t)
			tc.install(t, root, executable)
			before := clitest.TreeSnapshot(t, root)
			checks, err := doctorChecks(t, executable, "claude", "codex", tc.agent)
			if (tc.status == "FAIL") != errors.Is(err, cmdutil.ErrReported) || checks[tc.agent].Status != tc.status {
				t.Fatalf("installation trace hidden: %+v %v", checks[tc.agent], err)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("doctor changed installation traces")
			}
		})
	}
}

func TestDoctorVerboseCapabilities(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, executable := doctorAgentFixture(t)
	out, _, err := clitest.Execute(t, NewCmdWithExecutable(executable, "claude", "codex"), "--verbose")
	if err != nil || !strings.Contains(out, "delete guard:") || strings.Contains(out, "未限定") || strings.Contains(out, "实际验证: 未实机验证") {
		t.Fatalf("verbose capabilities: %s %v", out, err)
	}
	out, _, err = clitest.Execute(t, NewCmdWithExecutable(executable, "claude", "codex"), "--verbose", "--json")
	var checks []DoctorCheck
	if err != nil || json.Unmarshal([]byte(out), &checks) != nil {
		t.Fatal(out, err)
	}
	for _, name := range []string{"claude", "codex"} {
		var found bool
		for _, check := range checks {
			if check.Name != name {
				continue
			}
			found = true
			state, err := (initcmd.Planner{Executable: executable}).InspectAgent(name, integrate.Scope{Global: true})
			if err != nil || check.Capabilities == nil || *check.Capabilities != state.Capabilities || check.Installation != state.Installation {
				t.Fatalf("missing capability/status: %+v %+v %v", check, state, err)
			}
			raw, err := json.Marshal(check)
			if err != nil || !strings.Contains(string(raw), `"actuallyVerified":false`) {
				t.Fatalf("doctor conflated support and verification: %s %v", raw, err)
			}
		}
		if !found {
			t.Fatalf("missing %s in verbose checks", name)
		}
	}
}
