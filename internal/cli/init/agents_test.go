package init_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestShowRegisteredCapabilitiesWithoutInstalling(t *testing.T) {
	if testing.Short() {
		t.Skip("renders every registered agent and installation state")
	}
	root, executable := clitest.DoctorFixture(t)
	for _, agent := range []string{"claude", "codex"} {
		if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes"); err != nil {
			t.Fatal(err)
		}
	}
	oldHooks := filepath.Join(root, "codex", "hooks.json")
	data, err := os.ReadFile(oldHooks)
	if err != nil {
		t.Fatal(err)
	}
	clitest.WriteFixture(t, oldHooks, strings.ReplaceAll(string(data), filepath.ToSlash(filepath.Join(root, "bin", "obit.exe")), "C:/old/obit.exe"))
	before := clitest.TreeSnapshot(t, root)
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--show")
	if err != nil {
		t.Fatal(out, err)
	}
	summary, details, ok := strings.Cut(out, "能力明细 (capability details):")
	if !ok || !strings.HasPrefix(summary, "安装摘要 (installation summary):") || strings.Contains(summary, "delete guard:") || details == "" {
		t.Fatalf("summary missing or mixed with capabilities: %s", out)
	}
	for _, name := range initcmd.AgentNames() {
		if strings.Count(summary, "  "+name+": ") != 1 {
			t.Errorf("summary must have one line for %s: %s", name, summary)
		}
	}
	for _, want := range []string{"claude: 已安装，最新", "codex: 已安装，需要刷新", "windsurf: 未安装"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary missing %q: %s", want, summary)
		}
	}
	for _, name := range initcmd.AgentNames() {
		state, err := (initcmd.Planner{Executable: executable}).InspectAgent(name, integrate.Scope{Global: true})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, name+": ") || !strings.Contains(out, initcmd.CapabilityText(state.Capabilities)) {
			t.Fatalf("missing registered capabilities: %s\n%s", name, out)
		}
		if _, ok := integrate.LookupAgent(name); !ok {
			if !strings.Contains(out, name+": 安装尚未提供") {
				t.Fatalf("missing installer mistaken for failure: %s", out)
			}
			one, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--show", "--agent", name)
			if err != nil || !strings.Contains(one, "安装尚未提供") {
				t.Fatalf("cannot inspect adapter without installer: %s %v", one, err)
			}
			if strings.Contains(one, "安装摘要") {
				t.Fatal("filtered show unexpectedly included the inventory summary")
			}
		}
	}
	// A manual-only target remains missing until installation evidence exists.
	if !strings.Contains(out, "windsurf: 未安装；接入需要手动步骤") || !strings.Contains(out, "qoderwork: 未安装；接入需要手动步骤") {
		t.Fatalf("manual steps reported as failure: %s", out)
	}
	if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
		t.Fatal("show wrote installation files")
	}
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--show", "--agent", "unknown"); err == nil {
		t.Fatal("unknown agent accepted")
	}
}

func TestRegisteredAgentCLILifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range integrate.Agents() {
		for _, global := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/global=%v", agent.Name, global), func(t *testing.T) {
				root, repo, executable := agentFixture(t)
				if agent.Name == "claude" && global {
					testenv.FakeClaude(t)
				}
				args := []string{"--agent", agent.Name, "--yes"}
				if global {
					args = append(args, "-g")
				}
				if agent.Capabilities.MCP.Level != hook.Unsupported {
					args = append(args, "--mcp")
				}
				if agent.Capabilities.Skill.Level != hook.Unsupported && agent.PlanSkill != nil {
					args = append(args, "--skill")
				}
				manual := agent.Name == "openclaw" || agent.Name == "windsurf" || agent.Name == "codebuddy-ide" ||
					agent.Name == "qoderwork" || agent.Name == "lingma" || !global && agent.Name == "minimax-code" ||
					global && (agent.Name == "cursor" || agent.Name == "trae" || agent.Name == "trae-cn" || agent.Name == "roo") ||
					!global && (agent.Name == "kimi" || agent.Name == "hermes" || agent.Name == "cline")
				check := func(out string, err error) {
					t.Helper()
					if manual {
						if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(out, "未完成，需要手动步骤") || strings.Contains(out, "already installed") || strings.Contains(out, "installed; restart") {
							t.Fatalf("manual result: %s\n%v", out, err)
						}
					} else if err != nil {
						t.Fatalf("CLI result: %s\n%v", out, err)
					}
				}
				beforeRoot, beforeRepo := clitest.TreeSnapshot(t, root), clitest.TreeSnapshot(t, repo)
				out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, "--dry-run")...)
				check(out, err)
				if !reflect.DeepEqual(beforeRoot, clitest.TreeSnapshot(t, root)) || !reflect.DeepEqual(beforeRepo, clitest.TreeSnapshot(t, repo)) {
					t.Fatal("dry-run wrote files")
				}
				out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), args...)
				check(out, err)
				for name, want := range map[string]string{
					"opencode": "OBIT_OPENCODE_LOCAL=1", "openclaw": "OBIT_OPENCLAW_LOCAL=1",
					"pi": "OBIT_PI_LOCAL=1", "omp": "OBIT_OMP_LOCAL=1",
					"cline": "OBIT_CLINE_LOCAL_SHELL", "kilocode": "OBIT_KILOCODE_LOCAL_SHELL",
				} {
					if agent.Name == name && !strings.Contains(out, want) {
						t.Fatalf("missing runtime step %s: %s", want, out)
					}
				}
				installedRoot, installedRepo := clitest.TreeSnapshot(t, root), clitest.TreeSnapshot(t, repo)
				out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), args...)
				check(out, err)
				if !reflect.DeepEqual(installedRoot, clitest.TreeSnapshot(t, root)) || !reflect.DeepEqual(installedRepo, clitest.TreeSnapshot(t, repo)) {
					t.Fatal("repeated installation rewrote files")
				}
				out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, "--show")...)
				if err != nil || manual && !strings.Contains(out, "需要手动步骤") {
					t.Fatalf("show: %s %v", out, err)
				}
				if !reflect.DeepEqual(installedRoot, clitest.TreeSnapshot(t, root)) || !reflect.DeepEqual(installedRepo, clitest.TreeSnapshot(t, repo)) {
					t.Fatal("show wrote files")
				}
				// Refresh through the same CLI after relocating the executable.
				moved := filepath.Join(root, "relocated", "obit.exe")
				clitest.WriteFixture(t, moved, "relocated test executable")
				executable = func() (string, error) { return moved, nil }
				out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), args...)
				check(out, err)
				scope := integrate.Scope{Global: global, Project: repo}
				if !manual {
					for _, kind := range []string{"mcp", "skill"} {
						if kind == "skill" && agent.Capabilities.Skill.Level == hook.Unsupported {
							continue
						}
						exe, _ := executable.Path()
						st, err := integrate.InspectExtension(scope, agent.Name, kind, exe)
						if err != nil || !st.Installed || !st.Current || st.Manual {
							t.Fatalf("installed %s: %+v %v", kind, st, err)
						}
					}
				}
				out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, "--uninstall")...)
				check(out, err)
				if agent.Name != "openclaw" && (!global || agent.Name != "windsurf") {
					exe, _ := executable.Path()
					remaining, err := agent.Plan(scope, integrate.Commands{Native: executable.HookCommand(agent.Name), POSIX: executable.WSLHookCommand(agent.Name), Executable: exe}, true)
					if err != nil || integrate.Pending(remaining) {
						t.Fatalf("uninstall left automatic work: %+v %v", remaining, err)
					}
				}
			})
		}
	}
}

func TestAgentInstallationStates(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	for _, agent := range []string{"claude", "codex"} {
		scope := integrate.Scope{Global: true}
		status, err := (initcmd.Planner{Executable: executable}).InspectAgent(agent, scope)
		if err != nil || status.Installation != "missing" {
			t.Fatalf("fresh: %+v %v", status, err)
		}
		if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes"); err != nil {
			t.Fatal(err)
		}
		status, err = (initcmd.Planner{Executable: executable}).InspectAgent(agent, scope)
		if err != nil || status.Installation != "installed" {
			t.Fatalf("installed: %+v %v", status, err)
		}
		file := "settings.json"
		if agent == "codex" {
			file = "hooks.json"
		}
		path := filepath.Join(root, agent, file)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		clitest.WriteFixture(t, path, strings.ReplaceAll(string(data), filepath.ToSlash(filepath.Join(root, "bin", "obit.exe")), "C:/old/obit.exe"))
		status, err = (initcmd.Planner{Executable: executable}).InspectAgent(agent, scope)
		if err != nil || status.Installation != "outdated" {
			t.Fatalf("stale: %+v %v", status, err)
		}
	}
}

func TestCapabilityTextKeepsQualificationAndVerificationSeparate(t *testing.T) {
	t.Parallel()
	c := hook.Capabilities{Target: hook.Target{Host: "Host CLI", Distribution: "CN", Version: "v1", Scope: "user/project"}, DeleteGuard: hook.Support{Level: hook.Supported}, MCP: hook.Support{Level: hook.Limited, Target: hook.Target{Host: "Host IDE", Distribution: "global", Version: "v2", Scope: "explicit file"}}}
	text := initcmd.CapabilityText(c)
	for _, want := range []string{"delete guard: 支持", "宿主=Host CLI", "发行版=CN", "版本=v1", "作用域=user/project", "MCP: 有限", "宿主=Host IDE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "实际验证") {
		t.Fatalf("unverified capability includes verification detail: %s", text)
	}
	c.DeleteGuard.Verification = hook.Verification{Target: hook.Target{Host: "Host CLI", Version: "v1.1", Scope: "project"}, Evidence: "receipt example"}
	verified := initcmd.CapabilityText(c)
	if !strings.Contains(verified, "实际验证: 宿主=Host CLI, 版本=v1.1, 作用域=project; receipt example") || strings.Contains(verified, "未限定") {
		t.Fatal("verification evidence omitted or noisy", verified)
	}
}

func TestCapabilityTextOmitsUnspecifiedFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		target hook.Target
		want   string
	}{
		{"empty", hook.Target{}, "delete guard: 支持 (supported)"},
		{"host", hook.Target{Host: "CLI"}, "delete guard: 支持 (supported); 宿主=CLI"},
		{"distribution", hook.Target{Distribution: "CN"}, "delete guard: 支持 (supported); 发行版=CN"},
		{"version", hook.Target{Version: "v2"}, "delete guard: 支持 (supported); 版本=v2"},
		{"scope", hook.Target{Scope: "user"}, "delete guard: 支持 (supported); 作用域=user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := hook.Capabilities{Target: tc.target, DeleteGuard: hook.Support{Level: hook.Supported}}
			// Incomplete evidence continues to be omitted from human-readable output.
			c.DeleteGuard.Verification = hook.Verification{Evidence: "incomplete"}
			text := initcmd.CapabilityText(c)
			first, _, _ := strings.Cut(text, "\n")
			if first != tc.want || strings.Contains(text, "未限定") || strings.Contains(text, "实际验证") {
				t.Fatalf("capability text: %s", text)
			}
		})
	}
}

func TestManualStepsDoNotClaimInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, name := range []string{"cursor", "lingma", "openclaw", "windsurf", "qoderwork"} {
		t.Run(name, func(t *testing.T) {
			root, _, executable := agentFixture(t)
			before := clitest.TreeSnapshot(t, root)
			status, err := (initcmd.Planner{Executable: executable}).InspectAgent(name, integrate.Scope{Global: true})
			if err != nil || status.Installation != "missing" || !strings.Contains(status.Detail, "未安装；接入需要手动步骤") || strings.Contains(status.Detail, "已安装") {
				t.Fatalf("uninstalled manual target: %+v %v", status, err)
			}
			if name == "cursor" && (status.Capabilities.MCP.Level != hook.Supported || !strings.Contains(status.Detail, "Customize > Rules > User Rules")) {
				t.Fatal("installer detail or capability lost", status)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("inspection wrote configuration")
			}
		})
	}
}

func TestManualStatusFollowsInstallationEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, name := range []string{"cursor", "lingma", "openclaw", "windsurf", "qoderwork"} {
		t.Run(name, func(t *testing.T) {
			root, _, executable := agentFixture(t)
			out, _, _ := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", name, "--yes")
			before := clitest.TreeSnapshot(t, root)
			status, err := (initcmd.Planner{Executable: executable}).InspectAgent(name, integrate.Scope{Global: true})
			want := "manual"
			if name == "windsurf" || name == "qoderwork" {
				want = "missing"
			}
			if err != nil || status.Installation != want || strings.Contains(status.Detail, "已安装") {
				t.Fatalf("manual status after init: %+v %v\n%s", status, err, out)
			}
			if want == "manual" && (!strings.HasPrefix(status.Detail, "需要手动步骤") || !strings.Contains(status.Detail, "Next:")) {
				t.Fatalf("remaining manual steps missing: %+v", status)
			}
			out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--show", "--agent", name)
			if err != nil || !strings.Contains(out, name+": "+status.Detail) {
				t.Fatalf("show status disagrees: %s %v", out, err)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("inspection wrote configuration")
			}
		})
	}
}
