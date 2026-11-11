package init_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
)

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
			data := `{"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"C:/tools/obit.exe hook ` + agent + `"}]}]}}`
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
