//go:build windows

package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func TestDeleteDenyBashResurrection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	// Captured from Claude Code 2.1.317 on 2026-11-08 using the opt-in
	// protocol probe. Only machine paths and identifiers were redacted.
	data, err := os.ReadFile("testdata/claude-bash-pre-tool-use.json")
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
	}
	if err := json.Unmarshal(data, &input); err != nil || input.ToolName != "Bash" {
		t.Fatalf("captured input: %+v %v", input, err)
	}
	repo, _ := commandRepo(t)
	for _, path := range []string{"build/nested/keep", "a.log", "中文 b.log"} {
		writeFixture(t, filepath.Join(repo, path), "restorable "+path)
	}
	self := binaryPath(t, "obit")
	// The hook denies the original. Only a separate tool call runs the copied
	// suggestion; Bash really expands the wildcard and quoted variable.
	t.Setenv("OBIT_TEST_DELETE_TARGET", "build")
	input.ToolInput["command"] = `rm -r -- "$OBIT_TEST_DELETE_TARGET" *.log`
	data, _ = json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": input.ToolName, "tool_input": input.ToolInput})
	var denied bytes.Buffer
	probe := exec.Command(self, "hook", "claude")
	probe.Stdin, probe.Stdout = bytes.NewReader(data), &denied
	if err := probe.Run(); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Hook struct {
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(denied.Bytes(), &response); err != nil || response.Hook.Decision != "deny" {
		t.Fatalf("deny: %s %v", denied.String(), err)
	}
	parts := strings.Split(response.Hook.Reason, "`")
	if len(parts) < 3 {
		t.Fatalf("no copyable command: %s", response.Hook.Reason)
	}
	suggestion := parts[1]
	command := exec.Command(`C:\Program Files\Git\bin\bash.exe`, "--noprofile", "--norc", "-c", suggestion)
	command.Env = os.Environ()
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("separate suggested call failed: %s %v", out, err)
	}
	listing := exec.Command(self, "graveyard", "--json")
	listing.Env = command.Env
	out, err := listing.CombinedOutput()
	var graves struct{ Burials []core.Burial }
	if err != nil || json.Unmarshal(out, &graves) != nil || len(graves.Burials) != 3 {
		t.Fatalf("graveyard receipt: %s %v", out, err)
	}
	args := []string{"resurrect"}
	for _, b := range graves.Burials {
		args = append(args, fmt.Sprint(b.ID))
	}
	resurrect := exec.Command(self, args...)
	resurrect.Env = command.Env
	if out, err := resurrect.CombinedOutput(); err != nil {
		t.Fatalf("one-command resurrection: %s %v", out, err)
	}
	for _, path := range []string{"build/nested/keep", "a.log", "中文 b.log"} {
		got, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil || !bytes.Equal(got, []byte("restorable "+path)) {
			t.Errorf("not restored: %s %q %v", path, got, err)
		}
	}
}

func TestExpandedGlobOptionArgv(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Setenv("POSIXLY_CORRECT", "")
	if err := os.Unsetenv("POSIXLY_CORRECT"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASH_ENV", "")
	t.Setenv("LC_ALL", "C")
	for _, name := range []string{"rm", "git-clean"} {
		for _, option := range []string{"-n", "--help", "-x.log", "--exclude=keep.log"} {
			t.Run(name+"/"+option, func(t *testing.T) {
				repo, store := commandRepo(t)
				files := []string{option, "drop.log", "keep.log"}
				for _, file := range files {
					writeFixture(t, filepath.Join(repo, file), file)
				}
				// Expand once in real Bash, then give both implementations the
				// identical argv. No hook scan can hide a parser discrepancy.
				expand := exec.Command(`C:\Program Files\Git\bin\bash.exe`, "--noprofile", "--norc", "-c", `printf '%s\0' *`)
				out, err := expand.Output()
				if err != nil {
					t.Fatal(err)
				}
				argv := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
				if !slices.Equal(argv, files) {
					t.Fatalf("unexpected glob argv: %q", argv)
				}
				args := append([]string{"-f"}, argv...)
				command := "rm"
				reference := exec.Command(`C:\Program Files\Git\usr\bin\rm.exe`, args...)
				wantError := option != "--help"
				var wantBuried []string
				if name == "git-clean" {
					args[0], command = "-fdx", "git-clean"
					// Git's own dry run is the selection oracle; it never deletes
					// the reference files. Obit receives the original -fdx argv.
					reference = exec.Command("git", append([]string{"clean", "-n"}, args...)...)
					wantError = option == "--help" || option == "-x.log"
					opts, err := guardrm.ParseClean(args)
					if (err != nil) != wantError || !wantError && opts.DryRun != (option == "-n") {
						t.Fatalf("expanded Git option parsed incorrectly: %+v %v", opts, err)
					}
					if option == "--exclude=keep.log" {
						wantBuried = []string{"drop.log"}
					}
				}
				out, err = reference.CombinedOutput()
				if name == "git-clean" && option == "--help" {
					// Git versions differ: some return successful usage, others
					// reject --help after -n/-fdx. Both must leave the files alone;
					// Obit's parser and exit expectations below stay strict.
					if !strings.Contains(string(out), "usage: git clean") {
						t.Fatalf("reference did not report clean usage: %s %v", out, err)
					}
					var exit *exec.ExitError
					if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 129) {
						t.Fatalf("unexpected Git help exit: %s %v", out, err)
					}
				} else if (err != nil) != wantError {
					t.Fatalf("reference option result: %s %v", out, err)
				}
				if name == "git-clean" && !wantError {
					want := "Would remove drop.log\n"
					if option == "-n" {
						want += "Would remove keep.log\n"
					}
					if strings.ReplaceAll(string(out), "\r\n", "\n") != want {
						t.Fatalf("reference preview: %q, want %q", out, want)
					}
				}
				stdout, stderr, err := execute(t, command, args...)
				if (err != nil) != wantError {
					t.Errorf("obit option result differs: %s %s %v", stdout, stderr, err)
				}
				if name == "git-clean" && option == "-n" && stdout != string(out) {
					t.Errorf("dry-run preview differs: %q, want %q", stdout, out)
				}
				st, err := store.Read()
				if err != nil {
					t.Fatal(err)
				}
				var buried []string
				for _, burial := range st.Burials {
					buried = append(buried, filepath.Base(burial.Path))
				}
				if !slices.Equal(buried, wantBuried) {
					t.Errorf("selected files differ: %q, want %q", buried, wantBuried)
				}
				for _, file := range files {
					if !slices.Contains(wantBuried, file) {
						if data, err := os.ReadFile(filepath.Join(repo, file)); err != nil || string(data) != file {
							t.Errorf("unselected file changed: %s %q %v", file, data, err)
						}
					}
				}
			})
		}
	}
}

func TestRmHelpDoesNotBuryEarlierOperands(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, store := commandRepo(t)
	t.Setenv("POSIXLY_CORRECT", "")
	if err := os.Unsetenv("POSIXLY_CORRECT"); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(repo, "keep.log"), "keep")
	args := []string{"-f", "keep.log", "--help", "-x.log"}
	if out, err := exec.Command(`C:\Program Files\Git\usr\bin\rm.exe`, args...).CombinedOutput(); err != nil || !strings.Contains(string(out), "Usage:") {
		t.Fatalf("GNU help short circuit: %s %v", out, err)
	}
	if out, stderr, err := execute(t, "rm", args...); err != nil || !strings.Contains(out, "Usage:") {
		t.Errorf("obit help short circuit: %s %s %v", out, stderr, err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, "keep.log")); err != nil || string(data) != "keep" {
		t.Errorf("help changed earlier operand: %q %v", data, err)
	}
	if st, err := store.Read(); err != nil || len(st.Burials) != 0 {
		t.Fatalf("help recorded burials: %+v %v", st.Burials, err)
	}
}
