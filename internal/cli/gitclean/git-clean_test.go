package gitclean

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	resurrectcmd "github.com/woodfishhhh/obituary/internal/cli/resurrect"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestGitCleanRejectsOptionsBeforeAnyBurial(t *testing.T) {
	if testing.Short() {
		t.Skip("invalid arguments must be rejected in a usable Git worktree")
	}
	for _, args := range [][]string{
		{"-i"}, {"--interactive"}, {"--no-interactive"}, {"--no-force"}, {"--no-dry-run"}, {"--no-quiet"},
		{"--help"}, {"-h"}, {"--version"}, {"--unknown"}, {"-e"}, {"--exclude"}, {"-x", "-X"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			clitest.IsolateRunner(t)
			// A broken repository would hide a CLI that ignores parse errors.
			repo, store := clitest.CommandRepo(t)
			clitest.WriteFixture(t, filepath.Join(repo, "keep"), "keep")
			args = append([]string{"-f", "keep"}, args...)
			if _, _, err := clitest.Execute(t, NewCmd(), args...); err == nil {
				t.Error("unsupported, invalid or conflicting option accepted")
			}
			if got, err := os.ReadFile(filepath.Join(repo, "keep")); err != nil || string(got) != "keep" {
				t.Errorf("invalid options moved an earlier operand: %q %v", got, err)
			}
			if st, err := store.Read(); err != nil || len(st.Burials) != 0 {
				t.Errorf("invalid options recorded burials: %+v %v", st.Burials, err)
			}
			// -x and -X are individually supported; Git rejects their combination
			// during preview. Unsupported options must pass the suggestion gate.
			if args[len(args)-1] != "-X" {
				command := "git clean " + strings.Join(args, " ")
				if got, why := guardrm.Suggest(command, guardrm.Context{Shell: guardrm.Bash}); got != command || why == "" {
					t.Errorf("unsupported option suggested: %q (%s)", got, why)
				}
			}
		})
	}
}

func TestGitCleanRejectsMissingExcludeValue(t *testing.T) {
	for _, option := range []string{"-e", "--exclude"} {
		t.Run(option, func(t *testing.T) {
			repo, _ := clitest.CommandDir(t)
			clitest.WriteFixture(t, filepath.Join(repo, "keep"), "keep")
			_, stderr, err := clitest.Execute(t, NewCmd(), "-f", "keep", option)
			if err == nil || !strings.Contains(stderr, "option '"+option+"' requires a value") {
				t.Fatalf("missing exclude value was not rejected by the CLI parser: %s %v", stderr, err)
			}
		})
	}
}

func TestCleanPreviewPaths(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ output, want string }{
		{"Would remove loose.log\n", filepath.Join(root, "loose.log")},
		{"Would remove \"space file.log\"\n", filepath.Join(root, "space file.log")},
		{"Would remove ../keep.log\n", ""},
		{"Would remove .\n", ""},
		{"unknown output\n", ""},
	} {
		paths, err := cleanPaths(tc.output, root, "")
		if tc.want == "" {
			if err == nil {
				t.Fatalf("unsafe preview accepted: %q -> %v", tc.output, paths)
			}
		} else if err != nil || len(paths) != 1 || paths[0] != tc.want {
			t.Fatalf("preview %q: %v %v", tc.output, paths, err)
		}
	}
}

func TestGitPreviewRaceStaysRestorable(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandRepo(t)
	path := filepath.Join(repo, "loose")
	clitest.WriteFixture(t, path, "previewed content")
	old := cmdutil.Open
	// git-clean opens the service after Git's preview. Another process can
	// edit and stage the selected path during that gap.
	cmdutil.Open = func() (*core.Service, error) {
		clitest.WriteFixture(t, path, "edited after preview")
		if out, err := exec.Command("git", "add", "loose").CombinedOutput(); err != nil {
			t.Fatalf("concurrent git add: %s %v", out, err)
		}
		return old()
	}
	t.Cleanup(func() { cmdutil.Open = old })
	if _, _, err := clitest.Execute(t, NewCmd(), "-f"); err != nil {
		t.Fatal(err)
	}
	cmdutil.Open = old
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 {
		t.Fatalf("burials: %+v %v", st.Burials, err)
	}
	if _, _, err := clitest.Execute(t, resurrectcmd.NewCmd(), fmt.Sprint(st.Burials[0].ID)); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "edited after preview" {
		t.Fatalf("race lost data: %q %v", got, err)
	}
}
