package cmdutil_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	addcmd "github.com/woodfishhhh/obituary/internal/cli/add"
	burycmd "github.com/woodfishhhh/obituary/internal/cli/bury"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	mktempcmd "github.com/woodfishhhh/obituary/internal/cli/mktemp"
	proposalscmd "github.com/woodfishhhh/obituary/internal/cli/proposals"
	rulescmd "github.com/woodfishhhh/obituary/internal/cli/rules"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestRegistrationCommandsEnsureRunner(t *testing.T) {
	for _, name := range []string{"add", "approve", "mktemp", "apply", "bury"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(name+map[bool]string{true: " JSON", false: " text"}[asJSON], func(t *testing.T) {
				starts := clitest.IsolateRunner(t)
				repo, store := runnerCommandFixture(t, name)
				path := filepath.Join(repo, "debug.log")
				clitest.WriteFixture(t, path, "debug")
				var cmd *cobra.Command
				var args []string
				switch name {
				case "add":
					cmd, args = addcmd.NewCmd(), []string{path, "1d"}
				case "approve":
					clitest.AddProposal(t, store, repo, core.Candidate{Path: path, Lifespan: "1d"})
					cmd, args = proposalscmd.NewDecideCmd("approve", "", true), []string{"1"}
				case "mktemp":
					cmd, args = mktempcmd.NewCmd(), []string{"1d"}
				case "apply":
					cmd, args = rulescmd.NewCmd(), []string{"apply", path}
				case "bury":
					cmd, args = burycmd.NewCmd(), []string{path}
				}
				if asJSON {
					args = append(args, "--json")
				}
				out, stderr, err := clitest.Execute(t, cmd, args...)
				if err != nil {
					t.Fatalf("%v: %s %s", err, out, stderr)
				}
				if len(*starts) != 1 || (*starts)[0] != store.Home {
					t.Fatalf("runner homes: %v", *starts)
				}
				if asJSON {
					if !json.Valid([]byte(out)) || stderr != "" {
						t.Fatalf("JSON polluted: %q stderr=%q", out, stderr)
					}
				} else if !strings.Contains(stderr, "started a daemon") || strings.Contains(out, "started a daemon") {
					t.Fatalf("runner notice: stdout=%q stderr=%q", out, stderr)
				}
				state, err := store.Inspect()
				if err != nil || len(state.Items)+len(state.Burials) != 1 {
					t.Fatalf("registration did not persist: %+v: %v", state, err)
				}
			})
		}
	}
}

func TestNoRunnerForChecksSkippedPathsAndFailedRegistrations(t *testing.T) {
	for _, name := range []string{"check", "skipped", "approve", "bury", "mktemp", "reject"} {
		t.Run(name, func(t *testing.T) {
			starts := clitest.IsolateRunner(t)
			repo, store := runnerCommandFixture(t, name)
			path := filepath.Join(repo, "file.txt")
			clitest.WriteFixture(t, path, "keep")
			var cmd *cobra.Command
			var args []string
			switch name {
			case "check":
				path = filepath.Join(repo, "debug.log")
				clitest.WriteFixture(t, path, "debug")
				cmd, args = rulescmd.NewCmd(), []string{"check", path}
			case "skipped":
				cmd, args = rulescmd.NewCmd(), []string{"apply", path}
			case "approve":
				clitest.AddProposal(t, store, repo, core.Candidate{Path: filepath.Join(repo, "gone"), Lifespan: "1d"})
				cmd, args = proposalscmd.NewDecideCmd("approve", "", true), []string{"1"}
			case "bury":
				cmd, args = burycmd.NewCmd(), []string{filepath.Join(repo, "gone")}
			case "mktemp":
				cmd, args = mktempcmd.NewCmd(), []string{"invalid"}
			case "reject":
				clitest.AddProposal(t, store, repo, core.Candidate{Path: path, Lifespan: "1d"})
				cmd, args = proposalscmd.NewDecideCmd("reject", "", false), []string{"1"}
			}
			clitest.Execute(t, cmd, append(args, "--json")...)
			if len(*starts) != 0 {
				t.Fatalf("unexpected runner start: %v", *starts)
			}
		})
	}
}

func TestBuryAndRulesApplyStartRunnerAfterPartialSuccess(t *testing.T) {
	for _, name := range []string{"bury", "apply"} {
		t.Run(name, func(t *testing.T) {
			starts := clitest.IsolateRunner(t)
			repo, store := runnerCommandFixture(t, name)
			path := filepath.Join(repo, "debug.log")
			clitest.WriteFixture(t, path, "debug")
			args := []string{path, filepath.Join(repo, "missing"), "--json"}
			cmd := burycmd.NewCmd()
			if name == "apply" {
				cmd, args = rulescmd.NewCmd(), append([]string{"apply"}, args...)
			}
			out, stderr, err := clitest.Execute(t, cmd, args...)
			if (name == "bury") != errors.Is(err, cmdutil.ErrReported) || !json.Valid([]byte(out)) || stderr != "" {
				t.Fatalf("%v: %s %s", err, out, stderr)
			}
			if len(*starts) != 1 || (*starts)[0] != store.Home {
				t.Fatalf("partial success starts: %v", *starts)
			}
		})
	}
}

func runnerCommandFixture(t *testing.T, name string) (string, core.Store) {
	t.Helper()
	switch name {
	case "approve", "mktemp", "apply", "check", "skipped":
		if testing.Short() {
			t.Skip("registration and rule checks query a real Git repository")
		}
		return clitest.CommandRepo(t)
	default:
		return clitest.CommandDir(t)
	}
}

func TestEnsureDaemonUsesHomeAndKeepsJSONQuiet(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		running, serves, quiet, fail bool
		want                         int
	}{
		{"resident", true, false, false, false, 0},
		{"matching task", false, true, false, false, 0},
		{"other home", false, false, false, false, 1},
		{"JSON start failure", false, false, true, true, 1},
		{"text start failure", false, false, false, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			starts := clitest.IsolateRunner(t)
			cmdutil.RunnerInstalled = func() bool { return true }
			cmdutil.RunnerRunning = func(string) bool { return tc.running }
			cmdutil.RunnerServes = func(string) bool { return tc.serves }
			if tc.fail {
				cmdutil.RunnerStart = func(home string) error { *starts = append(*starts, home); return errors.New("test failure") }
			}
			out, stderr, err := clitest.Execute(t, &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) { cmdutil.EnsureDaemon("isolated", tc.quiet) }})
			if err != nil || out != "" || len(*starts) != tc.want || (tc.quiet && stderr != "") {
				t.Fatalf("starts=%v stdout=%q stderr=%q err=%v", *starts, out, stderr, err)
			}
			if tc.fail && !tc.quiet && !strings.Contains(stderr, "warning:") {
				t.Fatalf("missing warning: %q", stderr)
			}
		})
	}
}

// macOS and Linux have no resident daemon. The note says what does act on
// lifespans there, not that starting one is unsupported.
func TestEnsureDaemonExplainsMissingTimer(t *testing.T) {
	starts := clitest.IsolateRunner(t)
	cmdutil.RunnerRunning = func(string) bool { return false }
	cmdutil.RunnerServes = func(string) bool { return false }
	cmdutil.RunnerStart = func(home string) error {
		*starts = append(*starts, home)
		return fmt.Errorf("resident mode is unavailable: %w", core.ErrUnsupported)
	}
	_, stderr, err := clitest.Execute(t, &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) { cmdutil.EnsureDaemon("isolated", false) }})
	if err != nil || len(*starts) != 1 || !strings.Contains(stderr, "obit reap") || !strings.Contains(stderr, "obit daemon enable") ||
		strings.Contains(stderr, "could not start") || strings.Contains(stderr, core.ErrUnsupported.Error()) {
		t.Fatalf("stderr=%q err=%v", stderr, err)
	}
}
