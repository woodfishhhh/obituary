//go:build windows

package e2e_test

import (
	"errors"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestGitCleanRequireForceConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, value := range []string{"unset", "true", "false", "invalid"} {
		t.Run(value, func(t *testing.T) {
			repo, store := commandRepo(t)
			writeFixture(t, filepath.Join(repo, "loose"), "keep")
			if value != "unset" {
				if out, err := exec.Command("git", "config", "clean.requireForce", value).CombinedOutput(); err != nil {
					t.Fatalf("config: %s %v", out, err)
				}
			}
			_, stderr, err := execute(t, "git-clean", "-d")
			st, readErr := store.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if value == "false" {
				if err != nil || len(st.Burials) != 1 {
					t.Fatalf("requireForce=false: %s %v %+v", stderr, err, st.Burials)
				}
			} else if err == nil || len(st.Burials) != 0 {
				t.Fatalf("force required: %s %v %+v", stderr, err, st.Burials)
			}
		})
	}
}

func TestSuggestGlobOptionsRealGit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, _ := commandRepo(t)
	writeFixture(t, filepath.Join(repo, "-n"), "option-shaped filename")
	writeFixture(t, filepath.Join(repo, "keep.txt"), "keep")
	// The actual shell expansion turns * into -n keep.txt: Git only previews.
	cmd := exec.Command(`C:\Program Files\Git\bin\bash.exe`, "--noprofile", "--norc", "-c", "git clean -fdx *")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Would remove keep.txt") {
		t.Fatalf("real glob did not select dry-run: %s %v", out, err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("original dry-run changed keep.txt: %q %v", data, err)
	}
	for _, command := range []string{"git clean -fdx *", "rm *", "rmdir *"} {
		t.Run(command, func(t *testing.T) {
			if got, why := guardrm.Suggest(command, guardrm.Context{Shell: guardrm.Bash, Cwd: repo}); got != command || why == "" {
				t.Fatalf("option-producing glob suggested: %q (%s)", got, why)
			}
		})
	}
}

func TestGitCleanUsesRealPreview(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"force", []string{"-f"}, []string{"loose", "中文 空格"}},
		{"fd", []string{"-fd"}, []string{"loose", "nested", "中文 空格"}},
		{"fdx", []string{"-fdx"}, []string{"ignored", "loose", "nested", "中文 空格"}},
		{"ignored-only", []string{"-fdX"}, []string{"ignored"}},
		{"long-force", []string{"--force", "-d"}, []string{"loose", "nested", "中文 空格"}},
		{"quiet", []string{"-qfd"}, []string{"loose", "nested", "中文 空格"}},
		{"long-quiet", []string{"--quiet", "-fd"}, []string{"loose", "nested", "中文 空格"}},
		{"dry", []string{"-ndx"}, nil},
		{"long-dry", []string{"--dry-run", "-dx"}, nil},
		{"quiet-dry", []string{"-qndx"}, nil},
		{"exclude", []string{"-fdx", "-e", "loose"}, []string{"ignored", "nested", "中文 空格"}},
		{"exclude-attached", []string{"-fdx", "-eloose"}, []string{"ignored", "nested", "中文 空格"}},
		{"exclude-cluster", []string{"-fdxeloose"}, []string{"ignored", "nested", "中文 空格"}},
		{"exclude-long", []string{"-fdx", "--exclude", "loose"}, []string{"ignored", "nested", "中文 空格"}},
		{"exclude-equals", []string{"-fdx", "--exclude=loose"}, []string{"ignored", "nested", "中文 空格"}},
		{"exclude-ignored", []string{"-fdX", "-e", "loose"}, []string{"ignored", "loose"}},
		{"path", []string{"-fdx", "--", "nested"}, []string{"nested"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, store := commandRepo(t)
			writeFixture(t, filepath.Join(repo, ".gitignore"), "ignored\n")
			if out, err := exec.Command("git", "add", ".gitignore").CombinedOutput(); err != nil {
				t.Fatalf("git add: %s %v", out, err)
			}
			files := []string{"ignored", "loose", "nested/file", "中文 空格"}
			for _, path := range files {
				writeFixture(t, filepath.Join(repo, path), "restore me")
			}
			// Git selects paths itself; the adapter may only bury that selection.
			preview, err := exec.Command("git", append([]string{"clean", "-n"}, tc.args...)...).CombinedOutput()
			if err != nil {
				t.Fatalf("reference preview: %s %v", preview, err)
			}
			out, stderr, err := execute(t, "git-clean", tc.args...)
			if err != nil {
				t.Fatalf("git-clean: %s %s %v", out, stderr, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != len(tc.want) {
				t.Fatalf("burials = %+v, want %v: %v", st.Burials, tc.want, err)
			}
			for _, name := range tc.want {
				if _, err := os.Stat(filepath.Join(repo, name)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("not buried: %s %v", name, err)
				}
			}
			for _, file := range files {
				if !slices.Contains(tc.want, strings.SplitN(file, "/", 2)[0]) {
					if data, err := os.ReadFile(filepath.Join(repo, file)); err != nil || string(data) != "restore me" {
						t.Errorf("unselected file changed: %s %q %v", file, data, err)
					}
				}
			}
			if strings.Contains(tc.name, "dry") && out != string(preview) {
				t.Errorf("dry-run output differs: %q, want %q", out, preview)
			}
			if strings.Contains(tc.name, "quiet") && out != "" {
				t.Errorf("quiet output: %q", out)
			}
			if got, err := os.ReadFile(filepath.Join(repo, ".gitignore")); err != nil || string(got) != "ignored\n" {
				t.Fatalf("tracked file touched: %q %v", got, err)
			}
		})
	}
}

func TestGitCleanDoubleForceIncludesNestedRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, args := range [][]string{{"-fd"}, {"--force", "-d"}, {"-ffd"}, {"--force", "--force", "-d"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			repo, store := commandRepo(t)
			writeFixture(t, filepath.Join(repo, "nested", "keep"), "nested work")
			writeFixture(t, filepath.Join(repo, "loose"), "loose work")
			if out, err := exec.Command("git", "init", "-q", "nested").CombinedOutput(); err != nil {
				t.Fatalf("nested repository: %s %v", out, err)
			}
			preview, err := exec.Command("git", append([]string{"clean", "-n"}, args...)...).CombinedOutput()
			if err != nil {
				t.Fatalf("reference preview: %s %v", preview, err)
			}
			nested := strings.Contains(string(preview), "Would remove nested/")
			if _, stderr, err := execute(t, "git-clean", args...); err != nil {
				t.Fatalf("git-clean: %s %v", stderr, err)
			}
			want := 1
			if nested {
				want++
			} else if data, err := os.ReadFile(filepath.Join(repo, "nested", "keep")); err != nil || string(data) != "nested work" {
				t.Errorf("single force changed nested repository: %q %v", data, err)
			}
			if st, err := store.Read(); err != nil || len(st.Burials) != want {
				t.Fatalf("force selection: %+v %v, want %d burials", st.Burials, err, want)
			}
		})
	}
}

func TestGitCleanRejectsForeignWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"environment", "core.worktree"} {
		t.Run(mode, func(t *testing.T) {
			repo, store := commandRepo(t)
			other := t.TempDir()
			testenv.InitRepo(t, other)
			for _, root := range []string{repo, other} {
				writeFixture(t, filepath.Join(root, "victim"), root)
			}
			if mode == "environment" {
				t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
				t.Setenv("GIT_WORK_TREE", other)
			} else if out, err := exec.Command("git", "config", "core.worktree", other).CombinedOutput(); err != nil {
				t.Fatalf("git config: %s %v", out, err)
			}
			self := binaryPath(t, "obit")
			cmd := exec.Command(self, "git-clean", "-f")
			cmd.Env = os.Environ()
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() == 0 || !strings.Contains(string(out), "worktree") {
				t.Errorf("foreign worktree must fail with a diagnostic: %s %v", out, err)
			}
			for _, root := range []string{repo, other} {
				if data, err := os.ReadFile(filepath.Join(root, "victim")); err != nil || string(data) != root {
					t.Errorf("wrong worktree moved: %s: %q %v", root, data, err)
				}
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("unexpected burial: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestGitCleanSubdirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, store := commandRepo(t)
	writeFixture(t, filepath.Join(repo, "victim"), "root")
	writeFixture(t, filepath.Join(repo, "sub", "victim"), "sub")
	writeFixture(t, filepath.Join(repo, "sub", "tracked"), "tracked")
	if out, err := exec.Command("git", "add", "sub/tracked").CombinedOutput(); err != nil {
		t.Fatalf("git add: %s %v", out, err)
	}
	t.Chdir(filepath.Join(repo, "sub"))
	if _, stderr, err := execute(t, "git-clean", "-fd"); err != nil {
		t.Fatalf("git-clean: %s %v", stderr, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 1 || !strings.EqualFold(st.Burials[0].Path, filepath.Join(repo, "sub", "victim")) {
		t.Fatalf("wrong prefix: %+v %v", st.Burials, err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, "victim")); err != nil || string(data) != "root" {
		t.Fatalf("root sibling moved: %q %v", data, err)
	}
}
