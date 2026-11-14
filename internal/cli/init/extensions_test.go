package init_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestInitImplicitExtensionFailureKeepsHooks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, kind := range []string{"mcp", "skill", "cli"} {
		t.Run(kind, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			agent := "codex"
			path := filepath.Join(root, "codex", "config.toml")
			before := "[broken"
			if kind == "skill" {
				// Resolve through the public plan so the fixture follows redirected homes.
				plan, err := integrate.PlanSkill(integrate.Scope{Global: true}, agent, false)
				if err != nil {
					t.Fatal(err)
				}
				path = plan[0].Path
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				if kind == "cli" {
					agent = "claude"
					testenv.FakeClaude(t)
					t.Setenv("OBIT_FAKE_CLAUDE_MODE", "fail")
					path = filepath.Join(root, "claude", ".claude.json")
					before = testenv.NativeFixture(`{"mcpServers":{"obituary":{"type":"stdio","command":"C:/old/obit.exe","args":["mcp"]}}}`)
				}
				clitest.WriteFixture(t, path, before)
			}
			args := []string{"-g", "--agent", agent, "--yes"}
			if kind == "skill" {
				args = append(args, "--no-skill")
			}
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), args...)
			if err != nil {
				t.Fatalf("implicit extension failure blocked hooks: %s %v", out, err)
			}
			if kind != "skill" && !strings.Contains(strings.ToLower(out), "warning") {
				t.Fatalf("implicit extension failure missing warning: %s", out)
			}
			hooks := filepath.Join(root, agent, "hooks.json")
			if agent == "claude" {
				hooks = filepath.Join(root, agent, "settings.json")
			}
			data, err := os.ReadFile(hooks)
			if err != nil || !strings.Contains(string(data), "obit") {
				t.Fatalf("implicit extension failure blocked hooks: %s %v", data, err)
			}
			if kind != "skill" {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != before {
					t.Fatalf("implicit extension changed: %s %v", data, err)
				}
			}
			flag := "--mcp"
			if kind == "skill" {
				flag = "--skill"
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes", flag); err == nil {
				t.Fatal("explicit extension failure reported success")
			}
		})
	}
}

func TestInitGlobalRefreshesOnlyInstalledExtensions(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			testenv.FakeClaude(t)
			scope := integrate.Scope{Global: true}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes"); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []string{"mcp", "skill"} {
				st, err := integrate.InspectExtension(scope, agent, kind, filepath.Join(root, "bin", "obit.exe"))
				if err != nil || st.Installed != (kind == "skill") {
					t.Fatalf("default extension state %s: %+v %v", kind, st, err)
				}
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes", "--mcp", "--skill"); err != nil {
				t.Fatal(err)
			}
			plan, err := integrate.PlanSkill(scope, agent, true)
			if err != nil {
				t.Fatal(err)
			}
			skillPath := plan[0].Path
			clitest.WriteFixture(t, skillPath, "previous version")
			clitest.WriteFixture(t, filepath.Join(filepath.Dir(skillPath), ".obituary-skill"), fmt.Sprintf("obituary-skill-v1\n%x\n", sha256.Sum256([]byte("previous version"))))
			exe := filepath.Join(root, "new path", "obit.exe")
			executable = func() (string, error) { return exe, nil }
			checks, _ := doctorChecks(t, executable)
			if checks["mcp"].Status != "WARN" || checks["skill"].Status != "WARN" {
				t.Fatalf("outdated extensions not WARN: %+v %+v", checks["mcp"], checks["skill"])
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--yes"); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []string{"mcp", "skill"} {
				st, err := integrate.InspectExtension(scope, agent, kind, exe)
				wantCurrent := agent != "claude" || kind != "mcp"
				if err != nil || st.Current != wantCurrent {
					t.Fatalf("installed %s not refreshed: %+v %v", kind, st, err)
				}
			}
		})
	}
}

func TestInitClaudeRefreshRequiresExplicitMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, executable := clitest.DoctorFixture(t)
	_, log := testenv.FakeClaude(t)
	path := filepath.Join(root, "claude", ".claude.json")
	before := testenv.NativeFixture(`{"mcpServers":{"obituary":{"command":"C:/old/obit.exe","args":["mcp"],"env":{"OBIT_HOME":"D:/ledger"}}}}`)
	clitest.WriteFixture(t, path, before)
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--yes")
	data, readErr := os.ReadFile(path)
	if err != nil || readErr != nil || string(data) != before {
		t.Fatalf("implicit refresh replaced existing Claude registration: %s %v", data, err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("implicit refresh started Claude")
	}
	if !strings.Contains(out, "outdated") || !strings.Contains(out, "'remove'") || !strings.Contains(out, "'add'") {
		t.Fatalf("outdated registration lacks manual commands: %s", out)
	}
	out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--mcp", "--yes")
	data, readErr = os.ReadFile(path)
	if err != nil || readErr != nil || string(data) == before || !strings.Contains(out, "not atomic") {
		t.Fatalf("explicit refresh missing execution or warning: %s %s %v", data, out, err)
	}
	st, err := integrate.InspectExtension(integrate.Scope{Global: true}, "claude", "mcp", filepath.Join(root, "bin", "obit.exe"))
	if err != nil || !st.Current {
		t.Fatalf("explicit refresh not current: %+v %v", st, err)
	}
}

func TestInitClaudeMCPDryRunShowsCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, _ := clitest.DoctorFixture(t)
	_, log := testenv.FakeClaude(t)
	exe := filepath.Join(root, "path with spaces", "obit.exe")
	executable := cmdutil.Executable(func() (string, error) { return exe, nil })
	plan, err := integrate.PlanMCP(integrate.Scope{Global: true}, "claude", exe, false)
	if err != nil {
		t.Fatal(err)
	}
	want := integrate.CommandText(plan[0].Commands[0])
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--mcp", "--dry-run")
	if err != nil || !strings.Contains(out, want) {
		t.Fatalf("dry-run missing full command: %s %v", out, err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("dry-run executed CLI")
	}
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--mcp", "--yes"); err != nil {
		t.Fatal(err)
	}
	exe = filepath.Join(root, "new path", "obit.exe")
	plan, err = integrate.PlanMCP(integrate.Scope{Global: true}, "claude", exe, false)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--dry-run")
	if err != nil || !strings.Contains(out, integrate.CommandText(plan[0].Commands[0])) || !strings.Contains(out, integrate.CommandText(plan[0].Commands[1])) {
		t.Fatalf("refresh command sequence missing: %s %v", out, err)
	}
}

func TestInitUninstallPreservesBrokenExtensions(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, content := range []string{"{broken", `{"mcpServers":{"obituary":{"command":"user","args":[]}}}`} {
		repo, _ := clitest.CommandRepo(t)

		executable := cmdutil.Executable(func() (string, error) { return filepath.Join(repo, "obit.exe"), nil })

		if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--yes"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(repo, ".mcp.json")
		clitest.WriteFixture(t, path, content)
		out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--uninstall", "--yes")
		if err == nil {
			t.Fatal("incomplete uninstall returned success")
		}
		settings, _ := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
		if strings.Contains(string(settings), "obit") {
			t.Fatalf("extension failure prevented hook uninstall: %s; %s; %v", settings, out, err)
		}
		if got, readErr := os.ReadFile(path); readErr != nil || string(got) != content {
			t.Fatalf("uninstall changed broken/unowned extension: %q %v", got, readErr)
		}
		if !strings.Contains(out+err.Error(), "preserv") {
			t.Fatalf("preserved extension not reported: %s %v", out, err)
		}
	}
}

func TestInitUninstallContinuesAfterCLIFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root, executable := clitest.DoctorFixture(t)
	testenv.FakeClaude(t)
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--yes", "--mcp", "--skill"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_FAKE_CLAUDE_MODE", "fail")
	_, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--uninstall", "--yes")
	if err == nil || !strings.Contains(err.Error(), "fake stderr diagnostic") {
		t.Fatalf("CLI failure not reported: %v", err)
	}
	settings, _ := os.ReadFile(filepath.Join(root, "claude", "settings.json"))
	if strings.Contains(string(settings), "obit") {
		t.Fatal("CLI failure prevented hook uninstall")
	}
	if _, err := os.Stat(filepath.Join(root, "claude", "skills", "obituary", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("CLI failure prevented skill uninstall")
	}
	data, err := os.ReadFile(filepath.Join(root, "claude", ".claude.json"))
	if err != nil || !strings.Contains(string(data), "obituary") {
		t.Fatalf("failed CLI changed registration: %s %v", data, err)
	}
}

func TestInitGlobalRefreshPreservesUnownedExtensions(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	path := filepath.Join(root, "claude", ".claude.json")
	original := `{"mcpServers":{"obituary":{"command":"user","args":[]}}}`
	clitest.WriteFixture(t, path, original)
	skill := filepath.Join(root, "claude", "skills", "obituary", "SKILL.md")
	clitest.WriteFixture(t, skill, "user skill")
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--yes"); err == nil {
		t.Fatal("default skill install accepted an unowned file")
	}
	if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--yes", "--no-skill"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != original {
		t.Fatalf("user MCP changed: %s %v", got, err)
	}
	if got, err := os.ReadFile(skill); err != nil || string(got) != "user skill" {
		t.Fatalf("user skill changed: %s %v", got, err)
	}
}

func TestInitExtensionsOptInShowRefreshUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			repo, _ := clitest.CommandRepo(t)

			exe := filepath.Join(repo, "old", "obit.exe")
			executable := cmdutil.Executable(func() (string, error) { return exe, nil })

			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes"); err != nil {
				t.Fatal(err)
			}
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--show", "--mcp", "--skill")
			if err != nil || !strings.Contains(out, "mcp: not installed") || !strings.Contains(out, "skill: installed, current") {
				t.Fatalf("show planned instead of installed: %s %v", out, err)
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes", "--mcp", "--skill"); err != nil {
				t.Fatal(err)
			}
			before := clitest.TreeSnapshot(t, repo)
			out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--show")
			if err != nil || !strings.Contains(out, "mcp: installed, current") || !strings.Contains(out, "skill: installed, current") {
				t.Fatalf("installed: %s %v", out, err)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, repo)) {
				t.Fatal("show wrote files")
			}
			exe = filepath.Join(repo, "new", "obit.exe")
			out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--show")
			if err != nil || !strings.Contains(out, "mcp: installed, outdated") {
				t.Fatalf("outdated: %s %v", out, err)
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes", "--mcp"); err != nil {
				t.Fatal(err)
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes", "--uninstall"); err != nil {
				t.Fatal(err)
			}
			out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--show")
			if err != nil || !strings.Contains(out, "mcp: not installed") || !strings.Contains(out, "skill: not installed") {
				t.Fatalf("uninstall: %s %v", out, err)
			}
		})
	}
}

// The backups obit init leaves beside a user's files expire like Obituary's
// other backups, instead of piling up in the agent's configuration folder.
func TestInitBackupsGetALifespan(t *testing.T) {
	root, executable := clitest.DoctorFixture(t)
	settings := filepath.Join(root, "claude", "settings.json")
	clitest.WriteFixture(t, settings, `{"model": "user's choice"}`)
	if out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", "claude", "--no-skill", "--no-context-menu", "--no-shortcuts", "--yes"); err != nil {
		t.Fatal(out, err)
	}
	svc, err := app.Open()
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Items) != 1 || !strings.EqualFold(st.Items[0].Path, settings+".bak") || st.Items[0].Source != "init" {
		t.Fatalf("backup not registered: %+v", st.Items)
	}
	if left := time.Until(st.Items[0].ExpiresAt); left < ledger.BackupLifespan-time.Hour || left > ledger.BackupLifespan {
		t.Fatalf("backup lifespan %v", left)
	}
}
