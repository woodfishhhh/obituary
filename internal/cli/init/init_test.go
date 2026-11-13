package init_test

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
)

func TestInitUpdateNotice(t *testing.T) {
	clitest.CommandDir(t)
	_, executable := clitest.DoctorFixture(t)
	const notice = `Obituary checks for updates at most once a day, reading only public release information. You can turn off "Check for updates automatically" in Settings or set check = false under [update] in config.toml. Privacy: https://obituary.woodfish.site/docs/en/privacy`
	args := []string{"-g", "--agent", "claude,codex", "--no-context-menu", "--no-shortcuts"}
	for _, tc := range []struct {
		name      string
		flags     []string
		wantCount int
		wantError bool
	}{
		{"preview", []string{"--dry-run"}, 1, false},
		{"unconfirmed", nil, 0, true},
		{"install", []string{"--yes"}, 1, false},
		{"repeat", []string{"--yes"}, 0, false},
		{"repeat-preview", []string{"--dry-run"}, 1, false},
		{"show", []string{"--show"}, 0, false},
		{"uninstall-preview", []string{"--uninstall", "--dry-run"}, 0, false},
		{"uninstall", []string{"--uninstall", "--yes"}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, tc.flags...)...)
			if (err != nil) != tc.wantError {
				t.Fatalf("output: %s; error: %v", out, err)
			}
			if strings.Count(out, notice) != tc.wantCount {
				t.Fatalf("want %d notices: %s", tc.wantCount, out)
			}
			if tc.wantCount == 1 && !strings.HasSuffix(out, notice+"\n") {
				t.Fatalf("notice must be last: %s", out)
			}
			if tc.name == "install" {
				want := "installed; restart Claude Code\n" +
					"installed. Codex only runs hooks you have trusted: start Codex, run /hooks and trust Obituary's hooks (project hooks also need the project to be trusted).\n" + notice + "\n"
				if !strings.HasSuffix(out, want) {
					t.Fatalf("installation summary changed: %s", out)
				}
			}
		})
	}
}

func TestInitProjectKeepsSidecarsInData(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository")
	}
	repo, _ := clitest.CommandRepo(t)
	_, executable := clitest.DoctorFixture(t)
	clitest.WriteFixture(t, filepath.Join(repo, "CLAUDE.md"), "user Claude instructions\n")
	clitest.WriteFixture(t, filepath.Join(repo, "AGENTS.md"), "user agent instructions\n")
	before := clitest.TreeSnapshot(t, repo)
	args := []string{"--agent", "claude,codex", "--no-context-menu", "--no-shortcuts"}
	out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, "--dry-run")...)
	if err != nil || !reflect.DeepEqual(before, clitest.TreeSnapshot(t, repo)) {
		t.Fatal("preview changed project", out, err)
	}
	var installed map[string]string
	for i, action := range []string{"--yes", "--yes", "--uninstall"} {
		out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), append(args, action, "--yes")...)
		if err != nil {
			t.Fatal(out, err)
		}
		if i == 0 {
			installed = clitest.TreeSnapshot(t, repo)
		}
		if i == 1 && !reflect.DeepEqual(installed, clitest.TreeSnapshot(t, repo)) {
			t.Fatal("repeat init changed project files")
		}
		entries, err := os.ReadDir(repo)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), ".obituary-") || strings.Contains(entry.Name(), ".bak") || strings.HasSuffix(entry.Name(), ".lock") {
				t.Errorf("unexpected project sidecar: %s", entry.Name())
			}
		}
	}
	for name, want := range map[string]string{"CLAUDE.md": "user Claude instructions\n", "AGENTS.md": "user agent instructions\n"} {
		got, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil || string(got) != want {
			t.Fatalf("user content changed: %s %q %v", name, got, err)
		}
	}
}

func TestInitGlobalSkillDirectoriesStayClean(t *testing.T) {
	clitest.CommandDir(t)
	root, executable := clitest.DoctorFixture(t)
	args := []string{"-g", "--agent", "claude,codex", "--no-context-menu", "--no-shortcuts", "--yes"}
	for range 2 {
		out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), args...)
		if err != nil {
			t.Fatal(out, err)
		}
		if strings.Contains(out, "(Agent Skill →") {
			t.Error("duplicate skill target", out)
		}
		for _, dir := range []string{filepath.Join(root, "claude", "skills", "obituary"), filepath.Join(root, "home", ".agents", "skills", "obituary")} {
			if strings.Count(out, filepath.Join(dir, "SKILL.md")) != 1 {
				t.Error("skill target must appear once", out)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			want := []string{".obituary-skill", "SKILL.md"}
			if runtime.GOOS != "windows" && dir == filepath.Join(root, "claude", "skills", "obituary") {
				want = []string{".obituary-skill", ".obituary-skill.bak", "SKILL.md"}
			}
			if !reflect.DeepEqual(names, want) {
				t.Errorf("skill contains auxiliary files: %s %v", dir, names)
			}
		}
	}
}

func BenchmarkInitShow(b *testing.B) {
	root, executable := clitest.DoctorFixture(b)
	b.Chdir(root)
	b.ResetTimer()
	for b.Loop() {
		if _, _, err := clitest.Execute(b, initcmd.NewCmdWithExecutable(executable), "-g", "--show"); err != nil {
			b.Fatal(err)
		}
	}
}

func TestInitDryRunRejectsChangedWorkingDirectory(t *testing.T) {
	testenv.WindowsOnly(t, "Win32 extended paths and Windows namespace normalization")
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	if os.Getenv("OBIT_TEST_INIT_CHANGED_CWD") == "1" {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(cwd) != "repo." {
			t.Skipf("Windows normalized process cwd before init: %q", cwd)
		}
		out, _, err := clitest.Execute(t, initcmd.NewCmd(), "--dry-run")
		if err == nil || !strings.Contains(err.Error(), "Windows normalization changes") || strings.Contains(out, "settings.json") {
			t.Fatalf("init planned changes in neighbor: %s %v", out, err)
		}
		return
	}
	neighbor := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(neighbor, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	special := `\\?\` + neighbor + "."
	if err := os.Mkdir(special, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestInitDryRunRejectsChangedWorkingDirectory$", "-test.v")
	cmd.Dir, cmd.Env = special, append(os.Environ(), "OBIT_TEST_INIT_CHANGED_CWD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init child: %v %s", err, out)
	}
	if strings.Contains(string(out), "--- SKIP:") {
		t.Skip(string(out))
	}
}

func TestGuardInstallShowUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			repo, _ := clitest.CommandRepo(t)

			executable := cmdutil.Executable(func() (string, error) { return filepath.Join(repo, "obit.exe"), nil })

			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes"); err != nil {
				t.Fatal(err)
			}
			settings := filepath.Join(repo, ".claude", "settings.json")
			if agent == "codex" {
				settings = filepath.Join(repo, ".codex", "hooks.json")
			}
			data, err := os.ReadFile(settings)
			if err != nil || !strings.Contains(string(data), "--notice-only") {
				t.Fatalf("default must register advisory-only hook: %s %v", data, err)
			}
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--show", "--guard-rm")
			if err != nil || !strings.Contains(out, "delete guard: registered: no") || !strings.Contains(out, "matcher coverage: none") {
				t.Fatalf("show planned instead of actual: %s %v", out, err)
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes", "--guard-rm"); err != nil {
				t.Fatal(err)
			}
			out, _, err = clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--show")
			if err != nil || !strings.Contains(out, "delete guard: registered: yes") || !strings.Contains(out, "matcher coverage: complete") {
				t.Fatalf("show: %s %v", out, err)
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes"); err != nil {
				t.Fatal(err)
			}
			data, _ = os.ReadFile(settings)
			if strings.Count(string(data), `"PreToolUse"`) != 1 {
				t.Fatalf("refresh lost/duplicated guard: %s", data)
			}
			if _, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "--agent", agent, "--yes", "--uninstall"); err != nil {
				t.Fatal(err)
			}
			data, _ = os.ReadFile(settings)
			if strings.Contains(string(data), "PreToolUse") {
				t.Fatalf("uninstall left guard: %s", data)
			}
		})
	}
}

func TestGuardShowAndDoctorIneffectiveMatcher(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			root, executable := clitest.DoctorFixture(t)
			settings := filepath.Join(root, "claude", "settings.json")
			if agent == "codex" {
				settings = filepath.Join(root, "codex", "hooks.json")
			}
			data := testenv.NativeFixture(`{"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"C:/tools/obit.exe hook `) + agent + `"}]}]}}`
			clitest.WriteFixture(t, settings, data)
			out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--show", "--guard-rm")
			if err != nil || !strings.Contains(out, "registered: yes; matcher coverage: incomplete") {
				t.Fatalf("show overstated coverage: %s %v", out, err)
			}
			checks, _ := doctorChecks(t, executable)
			check := checks[agent+"-guard-rm"]
			if check.Status != "WARN" || !strings.Contains(check.Detail, "registered: yes; matcher coverage: incomplete") {
				t.Fatalf("doctor overstated coverage: %+v", check)
			}
			if got, err := os.ReadFile(settings); err != nil || string(got) != data {
				t.Fatal("read-only inspection changed settings", err)
			}
		})
	}
}

func TestGuardReportsRegistrationCoverageAndEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"claude", "codex"} {
		for _, setting := range []struct{ key, value, want string }{
			{"OBIT_GUARD_RM", "1", "current process environment: not disabled"},
			{"OBIT_GUARD_RM", "0", "current process environment: disabled by OBIT_GUARD_RM=0"},
			{"OBIT_DISABLED", "true", "current process environment: disabled by OBIT_DISABLED"},
		} {
			t.Run(agent+"/"+setting.key+setting.value, func(t *testing.T) {
				root, executable := clitest.DoctorFixture(t)
				t.Setenv("OBIT_GUARD_RM", "1")
				t.Setenv("OBIT_DISABLED", "0")
				if _, stderr, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--guard-rm", "--yes"); err != nil {
					t.Fatalf("fixture install: %s %v", stderr, err)
				}
				t.Setenv(setting.key, setting.value)
				before := clitest.TreeSnapshot(t, root)
				out, _, err := clitest.Execute(t, initcmd.NewCmdWithExecutable(executable), "-g", "--agent", agent, "--show")
				if err != nil {
					t.Fatal(err)
				}
				checks, _ := doctorChecks(t, executable)
				check := checks[agent+"-guard-rm"]
				for _, fragment := range []string{"registered: yes", "matcher coverage: complete", setting.want, "agent environment not inspected"} {
					if !strings.Contains(out, fragment) || !strings.Contains(check.Detail, fragment) {
						t.Errorf("missing status dimension %q: show=%s doctor=%+v", fragment, out, check)
					}
				}
				if setting.value != "1" && check.Status != "WARN" {
					t.Errorf("disabled guard not reported: %+v", check)
				}
				if after := clitest.TreeSnapshot(t, root); !reflect.DeepEqual(before, after) {
					t.Fatal("status inspection wrote files")
				}
			})
		}
	}
}
