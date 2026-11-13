package guardrm

import (
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSuggestTrailingWindowsNames(t *testing.T) {
	t.Parallel()
	for _, context := range []Context{{Shell: Bash}, {Shell: Bash, WSL: true}, {Shell: PowerShell}, {Shell: Uncertain}} {
		for _, operand := range []string{`keep.`, `'keep '`, `tree./keep`, `'tree /keep'`, `'tree.\keep'`, `tree../keep`, `keep./`, `'keep /'`} {
			command := "rm -f " + operand
			if context.Shell == PowerShell {
				command = "Remove-Item -Force " + operand
			}
			if got, why := Suggest(command, context); got != command || why == "" {
				t.Errorf("ambiguous Win32 name changed (%+v): %q -> %q (%s)", context, command, got, why)
			}
		}
	}
	// Parent navigation is validated by the native deletion commands.
	for _, operand := range []string{`./keep`, `tree/./keep`, `tree/.hidden`, `'keep space'`} {
		if got, why := Suggest("rm -f "+operand, Context{Shell: Bash}); got != "obit rm -f "+operand || why != "" {
			t.Errorf("ordinary path lost: %q (%s)", got, why)
		}
	}
}

func TestSuggestOptionOrdering(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`rm victim -rf`, `rm -r victim -f`, `rm victim --force`, `rm victim -- -rf`,
		`rmdir empty -v`, `POSIXLY_CORRECT= rm -rf victim`, `POSIXLY_CORRECT=1 rm -rf victim`,
		`FOO=1 POSIXLY_CORRECT='' rm victim`, `POSIXLY_CORRECT=; rm -rf victim`,
	} {
		if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
			t.Errorf("environment-dependent options changed: %q -> %q (%s)", command, got, why)
		}
	}
	for _, command := range []string{`rm -rf victim`, `rm -- victim -rf`, `rm -f -- -rf`} {
		if got, why := Suggest(command, Context{Shell: Bash}); got == command || why != "" {
			t.Errorf("unambiguous options lost: %q (%s)", got, why)
		}
	}
}

func TestSuggestTerminalDotOperands(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"rm -rf", "rmdir"} {
		for _, operand := range []string{".", "..", "empty/.", "empty/./", "empty/..", "empty/../"} {
			command := name + " " + operand
			if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
				t.Errorf("dot operand suggested: %q (%s)", got, why)
			}
		}
	}
}

func TestSuggestParentTraversal(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"rm -f ", "rmdir ", "git clean -fdx "} {
		for _, ref := range []string{"./missing/../keep.txt", "../keep", "a/b/../../keep", "'a\\..\\keep'"} {
			command := prefix + ref
			want := "obit " + strings.Replace(command, "git clean", "git-clean", 1)
			if got, why := Suggest(command, Context{Shell: Bash}); got != want || why != "" {
				t.Errorf("validated parent traversal lost: %q (%s), want %q", got, why, want)
			}
			if got, why := Suggest(command, Context{Shell: Bash, WSL: true}); got != command || why == "" {
				t.Errorf("WSL parent traversal suggested: %q (%s)", got, why)
			}
		}
	}
}

func TestDeleteDriveRelativeOperands(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"rm", "rmdir", "remove-item", "git-clean"} {
		for _, ref := range []string{"D:keep.txt", "D:", "c:dir/file"} {
			t.Run(name+"/"+ref, func(t *testing.T) {
				var err error
				switch name {
				case "rm", "rmdir":
					_, err = ParseRemove([]string{ref}, name == "rmdir")
				case "remove-item":
					_, err = ParseRemoveItem([]string{"-LiteralPath", ref})
				case "git-clean":
					_, err = ParseClean([]string{"-fdx", "--", ref})
				}
				if err == nil || !strings.Contains(err.Error(), "drive-relative") {
					t.Fatalf("drive-relative operand accepted: %v", err)
				}
			})
		}
	}
	for _, ref := range []string{`D:\keep.txt`, "D:/keep.txt", "keep.txt", "./keep.txt"} {
		if _, err := ParseRemove([]string{ref}, false); err != nil {
			t.Errorf("valid rm operand %q: %v", ref, err)
		}
		if _, err := ParseRemoveItem([]string{"-LiteralPath", ref}); err != nil {
			t.Errorf("valid Remove-Item operand %q: %v", ref, err)
		}
		if _, err := ParseClean([]string{"-fdx", "--", ref}); err != nil {
			t.Errorf("valid git-clean operand %q: %v", ref, err)
		}
	}
}

func TestSuggestDriveRelativePassesThrough(t *testing.T) {
	t.Parallel()
	for _, shell := range []Shell{Bash, PowerShell} {
		commands := []string{"rm D:keep", "rmdir D:", "git clean -fdx D:keep"}
		if shell == PowerShell {
			commands = []string{"Remove-Item -LiteralPath D:keep", "Remove-Item D:"}
		}
		for _, command := range commands {
			if got, why := Suggest(command, Context{Shell: shell}); got != command || why == "" {
				t.Errorf("drive-relative suggestion: %q (%s)", got, why)
			}
		}
	}
}

func TestDeleteReview6PathParsing(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"rm", "rmdir", "remove-item", "git-clean"} {
		for _, path := range []string{"/tmp/keep", "/c/keep", "//server/share/keep"} {
			t.Run(name+"/"+path, func(t *testing.T) {
				var err error
				var paths []string
				switch name {
				case "rm", "rmdir":
					var parsed RemoveOptions
					parsed, err = ParseRemove([]string{path}, name == "rmdir")
					paths = parsed.Paths
				case "remove-item":
					_, err = ParseRemoveItem([]string{"-LiteralPath", path})
				case "git-clean":
					var parsed CleanOptions
					parsed, err = ParseClean([]string{"-fdx", "--", path})
					paths = parsed.Paths
				}
				if runtime.GOOS != "windows" && name != "remove-item" {
					if err != nil || !slices.Equal(paths, []string{path}) {
						t.Fatalf("native absolute operand changed: %v %v", paths, err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "POSIX absolute") {
					t.Fatalf("must reject ambiguous path before filesystem/Git lookup: %v", err)
				}
			})
		}
	}
	for _, path := range []string{`keep"".txt`, `keep".txt`} {
		if _, err := ParseRemoveItem([]string{"-LiteralPath", path}); err == nil || !strings.Contains(err.Error(), "double quote") {
			t.Errorf("embedded quote must fail at argument boundary: %v", err)
		}
	}
}

func TestRemoveHelpOptionBoundary(t *testing.T) {
	for _, command := range []string{"rm -f --help keep.log", "rmdir --help empty"} {
		if got, why := Suggest(command, Context{Shell: Bash}); got != command || why == "" {
			t.Errorf("help must not be denied: %q (%s)", got, why)
		}
	}
	for _, posix := range []bool{false, true} {
		t.Run(map[bool]string{false: "gnu", true: "posix"}[posix], func(t *testing.T) {
			t.Setenv("POSIXLY_CORRECT", "")
			if !posix {
				if err := os.Unsetenv("POSIXLY_CORRECT"); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-f", "keep.log", "--help"}
			opts, err := ParseRemove(args, false)
			if err != nil || opts.Help == posix || posix && !slices.Equal(opts.Paths, args[1:]) {
				t.Fatalf("help option ordering: %+v %v", opts, err)
			}
			opts, err = ParseRemove([]string{"-f", "--", "--help"}, false)
			if err != nil || opts.Help || !slices.Equal(opts.Paths, []string{"--help"}) {
				t.Fatalf("option terminator lost: %+v %v", opts, err)
			}
		})
	}
}
