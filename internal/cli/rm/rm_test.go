package rm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	resurrectcmd "github.com/woodfishhhh/obituary/internal/cli/resurrect"
	rmdircmd "github.com/woodfishhhh/obituary/internal/cli/rmdir"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestRmBurialsAndResurrection(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(repo, "build", "nested", "a.log"), "nested payload")
	clitest.WriteFixture(t, filepath.Join(repo, "中文 空格.log"), "unicode payload")
	_, stderr, err := clitest.Execute(t, NewCmd(), "-rv", "build", "中文 空格.log")
	if err != nil {
		t.Fatalf("rm: %s %v", stderr, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 2 {
		t.Fatalf("burials = %+v: %v", st.Burials, err)
	}
	var ids []string
	for _, burial := range st.Burials {
		if _, err := os.Stat(burial.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("source still exists: %s %v", burial.Path, err)
		}
		ids = append(ids, fmt.Sprint(burial.ID))
	}
	if out, stderr, err := clitest.Execute(t, resurrectcmd.NewCmd(), ids...); err != nil {
		t.Fatalf("resurrect: %s %s %v", out, stderr, err)
	}
	for path, want := range map[string]string{"build/nested/a.log": "nested payload", "中文 空格.log": "unicode payload"} {
		got, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil || string(got) != want {
			t.Fatalf("restored %s = %q: %v", path, got, err)
		}
	}
}

func TestRmForceOnlySuppressesMissing(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprint(force), func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			clitest.WriteFixture(t, filepath.Join(repo, "dir", "keep"), "keep")
			args := []string{"missing"}
			if force {
				args = append([]string{"-f"}, args...)
			}
			out, stderr, err := clitest.Execute(t, NewCmd(), args...)
			if force && (err != nil || out+stderr != "") || !force && (err == nil || !strings.Contains(stderr, "rm: cannot remove 'missing': No such file or directory")) {
				t.Fatalf("force=%v: out=%q stderr=%q err=%v", force, out, stderr, err)
			}
			_, stderr, err = clitest.Execute(t, NewCmd(), "-f", "dir")
			if err == nil || !strings.Contains(stderr, "rm: cannot remove 'dir': Is a directory") {
				t.Fatalf("-f must not imply -r: %q %v", stderr, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("unexpected burial: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestRmForceNoOperandsAndEmptyOperand(t *testing.T) {
	clitest.CommandDir(t)
	for _, args := range [][]string{{"-f"}, {"-f", ""}} {
		if out, stderr, err := clitest.Execute(t, NewCmd(), args...); err != nil || out+stderr != "" {
			t.Fatalf("rm %q: %q %q %v", args, out, stderr, err)
		}
	}
}

func TestRmRespectsRecycleModeWithoutFallback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "keep")
			clitest.WriteFixture(t, path, "keep")
			clitest.WriteFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[graveyard]\nmode = 'recycle'\n")
			svc, err := app.Open()
			if err != nil {
				t.Fatal(err)
			}
			trash := filepath.Join(t.TempDir(), "simulated-recycle")
			svc.FS = removeRecycleFS{FS: svc.FS, destination: trash, fail: fail}
			old := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = old })
			_, stderr, err := clitest.Execute(t, NewCmd(), "-f", "keep")
			if (err != nil) != fail {
				t.Fatalf("recycle failure=%v: %q %v", fail, stderr, err)
			}
			kept := trash
			if fail {
				kept = path
			}
			if got, err := os.ReadFile(kept); err != nil || string(got) != "keep" {
				t.Fatalf("recycle lost data: %q %v", got, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("recycle unexpectedly used graveyard: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestRmRefusalNeverDeletes(t *testing.T) {
	for _, fault := range []string{"protected", "repository", "identity"} {
		t.Run(fault, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			path := filepath.Join(repo, "victim")
			file := path
			if fault == "repository" {
				file = filepath.Join(path, ".git", "keep")
			}
			clitest.WriteFixture(t, file, "irreplaceable")
			svc, err := app.Open()
			if err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "protected":
				svc.Guard.Keep = append(svc.Guard.Keep, path)
			case "identity":
				if err := os.Link(path, filepath.Join(repo, "other-link")); err != nil {
					t.Fatal(err)
				}
			}
			old := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = old })
			args := []string{"-r", path}
			if fault != "repository" {
				args[0] = "-rf"
			}
			_, stderr, err := clitest.Execute(t, NewCmd(), args...)
			if err == nil || !strings.Contains(stderr, "rm: cannot remove '") {
				t.Errorf("refusal must report failure: %q %v", stderr, err)
			}
			got, readErr := os.ReadFile(file)
			if readErr != nil || string(got) != "irreplaceable" {
				t.Errorf("refusal deleted data: %q %v", got, readErr)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("refused object entered graveyard: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestRmPartialSuccessAndTrackedEdits(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandRepo(t)
	path := filepath.Join(repo, "tracked.txt")
	clitest.WriteFixture(t, path, "staged")
	if out, err := exec.Command("git", "add", "tracked.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %s %v", out, err)
	}
	clitest.WriteFixture(t, path, "uncommitted edit")
	_, stderr, err := clitest.Execute(t, NewCmd(), "missing", "tracked.txt")
	if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(stderr, "No such file") {
		t.Fatalf("partial failure: %s %v", stderr, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 {
		t.Fatalf("tracked file not buried: %+v %v", st.Burials, err)
	}
	if _, _, err := clitest.Execute(t, resurrectcmd.NewCmd(), fmt.Sprint(st.Burials[0].ID)); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "uncommitted edit" {
		t.Fatalf("uncommitted edit lost: %q %v", got, err)
	}
}

func TestRmRefusesDotOperands(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(repo, "child", "keep"), "keep")
	_, stderr, err := clitest.Execute(t, NewCmd(), "-rf", "child/.")
	if err == nil || !strings.Contains(stderr, "'.' or '..'") {
		t.Errorf("dot operand accepted: %q %v", stderr, err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, "child", "keep")); err != nil || string(got) != "keep" {
		t.Errorf("dot operand moved: %q %v", got, err)
	}
}

func TestRmOptionsAndEmptyDirectories(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, args := range [][]string{{"-d", "empty"}, {"--dir", "empty"}, {"-R", "empty"}, {"--recursive", "--force", "--preserve-root", "empty"}, {"--", "-f"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			if err := os.Mkdir(filepath.Join(repo, "empty"), 0o700); err != nil {
				t.Fatal(err)
			}
			clitest.WriteFixture(t, filepath.Join(repo, "-f"), "payload")
			out, stderr, err := clitest.Execute(t, NewCmd(), args...)
			if err != nil || out+stderr != "" {
				t.Fatalf("rm: %q %q %v", out, stderr, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 1 {
				t.Fatalf("burials=%+v %v", st.Burials, err)
			}
		})
	}
}

func TestRmRejectsUnsupportedOptionsBeforeAnyBurial(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Setenv("POSIXLY_CORRECT", "")
	if err := os.Unsetenv("POSIXLY_CORRECT"); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-i", "-I", "--interactive", "--interactive=never", "--interactive=once", "--interactive=always", "--one-file-system", "--preserve-root=all", "--no-preserve-root", "--version", "--unknown"} {
		t.Run(flag, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			clitest.WriteFixture(t, filepath.Join(repo, "keep"), "keep")
			for _, args := range [][]string{{flag, "keep"}, {"keep", flag}} {
				if _, stderr, err := clitest.Execute(t, NewCmd(), args...); err == nil || !strings.Contains(stderr, "unsupported option") {
					t.Errorf("unsupported option accepted: %q %v", stderr, err)
				}
			}
			if got, err := os.ReadFile(filepath.Join(repo, "keep")); err != nil || string(got) != "keep" {
				t.Errorf("argument before invalid option moved: %q %v", got, err)
			}
			if st, err := store.Read(); err != nil || len(st.Burials) != 0 {
				t.Errorf("unsupported option recorded burials: %+v %v", st.Burials, err)
			}
			command := "rm " + flag + " keep"
			if got, why := guardrm.Suggest(command, guardrm.Context{Shell: guardrm.Bash}); got != command || !strings.Contains(why, "unsupported option") {
				t.Errorf("unsupported option suggested: %q (%s)", got, why)
			}
		})
	}
}

func TestRemoveVerboseAndHelpOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, name := range []string{"rm", "rmdir"} {
		for _, flag := range []string{"-v", "--verbose", "--help"} {
			t.Run(name+"/"+flag, func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandDir(t)
				if err := os.Mkdir(filepath.Join(repo, "empty"), 0o700); err != nil {
					t.Fatal(err)
				}
				cmd, args := rmdircmd.NewCmd(), []string{flag, "--", "empty"}
				if name == "rm" {
					cmd, args = NewCmd(), append([]string{"-d"}, args...)
				}
				out, stderr, err := clitest.Execute(t, cmd, args...)
				want, text := 1, "buried"
				if flag == "--help" {
					want, text = 0, "Usage:"
				}
				if err != nil || !strings.Contains(out, text) {
					t.Errorf("option output: %q %q %v", out, stderr, err)
				}
				if st, err := store.Read(); err != nil || len(st.Burials) != want {
					t.Errorf("option burials: %+v %v", st.Burials, err)
				}
			})
		}
	}
}
