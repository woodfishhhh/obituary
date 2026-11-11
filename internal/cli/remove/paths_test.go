package remove_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	gitcleancmd "github.com/woodfishhhh/obituary/internal/cli/gitclean"
	removeitemcmd "github.com/woodfishhhh/obituary/internal/cli/removeitem"
	rmcmd "github.com/woodfishhhh/obituary/internal/cli/rm"
	rmdircmd "github.com/woodfishhhh/obituary/internal/cli/rmdir"
)

func TestRemoveRejectsTerminalDotOperands(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, name := range []string{"rm", "rmdir"} {
		for _, ref := range []string{"empty/.", "empty/./", `empty\.`, "empty/..", "empty/../", `empty\..`} {
			t.Run(name+"/"+ref, func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandDir(t)
				if err := os.Mkdir(filepath.Join(repo, "empty"), 0o700); err != nil {
					t.Fatal(err)
				}
				cmd, args := rmcmd.NewCmd(), []string{"-rf", ref}
				if name == "rmdir" {
					cmd, args = rmdircmd.NewCmd(), []string{ref}
				}
				_, stderr, err := clitest.Execute(t, cmd, args...)
				if err == nil || !strings.Contains(stderr, "'.' or '..'") {
					t.Errorf("dot operand not rejected before normalization: %s %v", stderr, err)
				}
				if _, err := os.Stat(filepath.Join(repo, "empty")); err != nil {
					t.Errorf("original directory lost: %v", err)
				}
				st, err := store.Read()
				if err != nil || len(st.Burials) != 0 {
					t.Fatalf("dot operand buried: %+v %v", st.Burials, err)
				}
			})
		}
	}
}

func TestRmMissingParentKeepsFile(t *testing.T) {
	for _, spelling := range []string{"slash", "backslash", "absolute"} {
		t.Run(spelling, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			clitest.WriteFixture(t, filepath.Join(repo, "keep.txt"), "keep")
			ref := "./missing/../keep.txt"
			switch spelling {
			case "backslash":
				ref = `.\missing\..\keep.txt`
			case "absolute":
				ref = repo + `/missing/../keep.txt` // Join would erase the missing prefix.
			}
			_, stderr, err := clitest.Execute(t, rmcmd.NewCmd(), "-f", ref)
			if err == nil || !strings.Contains(stderr, "rm: cannot remove '"+ref+"': No such file or directory") {
				t.Errorf("invalid parent traversal not rejected: %s %v", stderr, err)
			}
			if data, err := os.ReadFile(filepath.Join(repo, "keep.txt")); err != nil || string(data) != "keep" {
				t.Errorf("collapsed path selected keep.txt: %q %v", data, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("invalid parent traversal buried a file: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestDeleteParentTraversal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, name := range []string{"rm", "rmdir", "remove-item", "git-clean"} {
		for _, kind := range []string{"missing", "file", "junction", "ancestor-junction", "nested-missing", "valid"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandRepo(t)
				for _, dir := range []string{"keep", "real/child", "prefix"} {
					if err := os.MkdirAll(filepath.Join(repo, dir), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				ref := "./missing/../keep"
				switch kind {
				case "file":
					clitest.WriteFixture(t, filepath.Join(repo, "file"), "not a directory")
					ref = "./file/../keep"
				case "junction", "ancestor-junction":
					link := filepath.Join(repo, "link")
					if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Join(repo, "real")).CombinedOutput(); err != nil {
						t.Fatalf("mklink: %s %v", out, err)
					}
					ref = "./link/../keep"
					if kind == "ancestor-junction" {
						ref = "./link/child/../../keep"
					}
				case "nested-missing":
					ref = "./prefix/missing/../../keep"
				case "valid":
					ref = "./prefix/../keep"
				}
				cmd, args := rmcmd.NewCmd(), []string{"-rf", ref}
				switch name {
				case "rmdir":
					cmd, args = rmdircmd.NewCmd(), []string{ref}
				case "remove-item":
					cmd, args = itemCommand(), []string{"-LiteralPath", ref}
				case "git-clean":
					cmd, args = gitcleancmd.NewCmd(), []string{"-fdx", "--", ref}
				}
				_, stderr, err := clitest.Execute(t, cmd, args...)
				wantBurials := 0
				if kind == "valid" {
					wantBurials = 1
					if err != nil {
						t.Errorf("existing directory traversal refused: %s %v", stderr, err)
					}
				} else {
					if err == nil || !strings.Contains(stderr, "No such file or directory") {
						t.Errorf("unsafe traversal not rejected: %s %v", stderr, err)
					}
					if _, err := os.Stat(filepath.Join(repo, "keep")); err != nil {
						t.Errorf("normalized target lost: %v", err)
					}
				}
				st, err := store.Read()
				if err != nil || len(st.Burials) != wantBurials {
					t.Fatalf("burials=%+v, want %d: %v", st.Burials, wantBurials, err)
				}
			})
		}
	}
}

func TestDeleteRejectsPOSIXAbsolutePaths(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, name := range []string{"rm", "rmdir", "remove-item", "git-clean"} {
		t.Run(name, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandRepo(t)
			keep := filepath.Join(repo, "keep")
			if err := os.Mkdir(keep, 0o700); err != nil {
				t.Fatal(err)
			}
			// The raw single-slash spelling resolves to a real, otherwise
			// deletable target on this drive; absence/Guard cannot hide the gate.
			ref := filepath.ToSlash(keep[len(filepath.VolumeName(keep)):])
			cmd, args := rmcmd.NewCmd(), []string{"-rf", "keep", ref}
			switch name {
			case "rmdir":
				cmd, args = rmdircmd.NewCmd(), []string{"keep", ref}
			case "remove-item":
				cmd, args = itemCommand(), []string{"-LiteralPath", ref}
			case "git-clean":
				cmd, args = gitcleancmd.NewCmd(), []string{"-fdx", "--", "keep", ref}
			}
			if _, stderr, err := clitest.Execute(t, cmd, args...); err == nil || !strings.Contains(stderr, "POSIX absolute") {
				t.Errorf("ambiguous argv not rejected: %s %v", stderr, err)
			}
			if _, err := os.Stat(keep); err != nil {
				t.Errorf("native rooted path selected keep: %v", err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("invalid path buried files: %+v %v", st.Burials, err)
			}
		})
	}
}

func TestDeleteCommandsDoNotTranslateWSL(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, name := range []string{"rm", "rmdir", "remove-item", "git-clean"} {
		t.Run(name, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandRepo(t)
			path := filepath.Join(repo, "victim")
			if name == "rmdir" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				clitest.WriteFixture(t, path, "restore me")
			}
			wsl := "/mnt/" + strings.ToLower(path[:1]) + filepath.ToSlash(path[2:])
			cmd, args := rmcmd.NewCmd(), []string{wsl}
			switch name {
			case "rmdir":
				cmd = rmdircmd.NewCmd()
			case "remove-item":
				cmd, args = removeitemcmd.NewCmd(), []string{"-LiteralPath", wsl}
			case "git-clean":
				cmd, args = gitcleancmd.NewCmd(), []string{"-f", "--", wsl}
			}
			_, _, _ = clitest.Execute(t, cmd, args...)
			st, err := store.Read()
			if err != nil || len(st.Burials) != 0 {
				t.Fatalf("WSL spelling selected a Windows path: %+v %v", st.Burials, err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("unselected Windows path moved", err)
			}
		})
	}
}

func TestRemovePreservesMSYSRoots(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, name := range []string{"rm", "rmdir", "git-clean"} {
		for _, marker := range []string{"msys-2.0.dll", "bash.exe"} {
			t.Run(name+"/"+marker, func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandRepo(t)
				root := filepath.Join(repo, "PortableGit")
				file := filepath.Join(root, "usr", "bin", marker)
				clitest.WriteFixture(t, file, "keep runtime")
				if marker == "bash.exe" {
					if err := os.Mkdir(filepath.Join(root, "etc"), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				cmd, args := rmcmd.NewCmd(), []string{"-rf", root}
				switch name {
				case "rmdir":
					cmd, args = rmdircmd.NewCmd(), []string{root}
				case "git-clean":
					cmd, args = gitcleancmd.NewCmd(), []string{"-fdx", "--", "PortableGit"}
				}
				_, stderr, err := clitest.Execute(t, cmd, args...)
				want := name + ": it is dangerous to operate recursively on '" + root + "' (same as '/')"
				if !errors.Is(err, cmdutil.ErrReported) || !strings.Contains(stderr, want) {
					t.Errorf("MSYS root not refused: %q %v", stderr, err)
				}
				if data, err := os.ReadFile(file); err != nil || string(data) != "keep runtime" {
					t.Errorf("MSYS runtime moved: %q %v", data, err)
				}
				if st, err := store.Read(); err != nil || len(st.Burials) != 0 {
					t.Errorf("MSYS root entered graveyard: %d burials, %v", len(st.Burials), err)
				}
			})
		}
	}
}

func TestDeleteRejectsTrailingWindowsNames(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, name := range []string{"rm", "rmdir", "remove-item"} {
		for _, ref := range []string{"keep.", "keep ", "tree./keep", "tree /keep", `tree.\keep`, "keep./", "keep /"} {
			t.Run(name+"/"+ref, func(t *testing.T) {
				clitest.IsolateRunner(t)
				repo, store := clitest.CommandDir(t)
				for _, path := range []string{"keep", "tree/keep"} {
					if err := os.MkdirAll(filepath.Join(repo, path), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				command, args := rmcmd.NewCmd(), []string{"-rf", ref}
				switch name {
				case "rmdir":
					command, args = rmdircmd.NewCmd(), []string{ref}
				case "remove-item":
					command, args = itemCommand(), []string{"-Recurse", "-Force", ref}
				}
				_, stderr, err := clitest.Execute(t, command, args...)
				if err == nil || !strings.Contains(stderr, "trailing dot or space") {
					t.Errorf("ambiguous path was not rejected: %s %v", stderr, err)
				}
				for _, path := range []string{"keep", "tree/keep"} {
					if _, err := os.Stat(filepath.Join(repo, path)); err != nil {
						t.Errorf("normalized sibling moved: %s %v", path, err)
					}
				}
				st, err := store.Read()
				if err != nil || len(st.Burials) != 0 {
					t.Fatalf("ambiguous path buried: %+v %v", st.Burials, err)
				}
			})
		}
	}
}

func TestRmMissingDottedNameKeepsUndottedFile(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, store := clitest.CommandDir(t)
	clitest.WriteFixture(t, filepath.Join(repo, "keep"), "irreplaceable")
	if _, _, err := clitest.Execute(t, rmcmd.NewCmd(), "-f", "keep."); err == nil {
		t.Error("ambiguous spelling accepted")
	}
	if data, err := os.ReadFile(filepath.Join(repo, "keep")); err != nil || string(data) != "irreplaceable" {
		t.Errorf("keep was selected by keep.: %q %v", data, err)
	}
	st, err := store.Read()
	if err != nil || len(st.Burials) != 0 {
		t.Fatalf("wrong file buried: %+v %v", st.Burials, err)
	}
}

func TestRmPOSIXLYCorrect(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, mode := range []string{"unset", "empty", "1"} {
		t.Run(mode, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			t.Setenv("POSIXLY_CORRECT", "")
			switch mode {
			case "unset":
				if err := os.Unsetenv("POSIXLY_CORRECT"); err != nil {
					t.Fatal(err)
				}
			case "1":
				t.Setenv("POSIXLY_CORRECT", "1")
			}
			clitest.WriteFixture(t, filepath.Join(repo, "victim/keep"), "keep")
			clitest.WriteFixture(t, filepath.Join(repo, "-rf"), "operand")
			_, stderr, err := clitest.Execute(t, rmcmd.NewCmd(), "victim", "-rf")
			want := "victim"
			if mode != "unset" {
				want = "-rf"
				if err == nil || !strings.Contains(stderr, "Is a directory") {
					t.Errorf("POSIX options parsed after operand: %s %v", stderr, err)
				}
				if data, err := os.ReadFile(filepath.Join(repo, "victim/keep")); err != nil || string(data) != "keep" {
					t.Errorf("POSIX victim moved: %q %v", data, err)
				}
			} else if err != nil {
				t.Errorf("GNU default permutation failed: %s %v", stderr, err)
			}
			st, err := store.Read()
			if err != nil || len(st.Burials) != 1 || filepath.Base(st.Burials[0].Path) != want {
				t.Fatalf("option permutation selected wrong target: %+v %v; want %s", st.Burials, err, want)
			}
		})
	}
}
