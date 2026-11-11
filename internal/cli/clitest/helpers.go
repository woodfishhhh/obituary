package clitest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

// ReportHookLatency shares the production targets between handler and process
// measurements. Shared Windows worker load makes these diagnostic, not failures.
func ReportHookLatency(t testing.TB, name, scope string, samples []time.Duration) {
	t.Helper()
	p50Budget, p95Budget := 100*time.Millisecond, 150*time.Millisecond
	if strings.HasPrefix(name, "PreToolUse/") {
		p50Budget, p95Budget = 40*time.Millisecond, 0 // no PreToolUse p95 target
	}
	slices.Sort(samples)
	p50 := samples[(len(samples)-1)/2]/2 + samples[len(samples)/2]/2
	p95 := samples[(95*len(samples)+99)/100-1]
	t.Logf("hook latency %s (%s, n=%d): p50-ms=%.3f p95-ms=%.3f; p50 budget=%s p95 budget=%s",
		name, scope, len(samples), float64(p50)/float64(time.Millisecond), float64(p95)/float64(time.Millisecond), p50Budget, p95Budget)
	if p50 > p50Budget || p95Budget > 0 && p95 > p95Budget {
		t.Logf("HOOK LATENCY ABOVE TARGET: %s; inspect worker load and profile this path", name)
	}
}

func Execute(t testing.TB, cmd *cobra.Command, args ...string) (string, string, error) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, stderr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	if _, err := out.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := stderr.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(out)
	if err != nil {
		t.Fatal(err)
	}
	e, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), string(e), runErr
}

func WriteFixture(t testing.TB, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func CommandRepo(t *testing.T) (string, core.Store) {
	t.Helper()
	repo, store := CommandDir(t)
	testenv.InitRepo(t, repo)
	return repo, store
}

// CommandDir is sufficient for commands whose contract does not involve Git.
func CommandDir(t *testing.T) (string, core.Store) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	// Discovery and project-root guards only inspect this marker. Tests that
	// execute Git use CommandRepo, which fills it with a real repository.
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	home := filepath.Join(root, "data")
	t.Setenv("OBIT_HOME", home)
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	return repo, core.Store{Home: home}
}

// Runner functions are the process/OS boundary; command tests exercise actual
// files, Git repositories and stores while keeping OS runner state untouched.
func IsolateRunner(t testing.TB) *[]string {
	t.Helper()
	menu := contextmenu.Current
	contextmenu.Current = NewMenuRegistry()
	t.Cleanup(func() { contextmenu.Current = menu })
	startup := runkey.Current
	runkey.Current = testenv.NewRunRegistry()
	t.Cleanup(func() { runkey.Current = startup })
	running, start, autostart := cmdutil.RunnerRunning, cmdutil.RunnerStart, cmdutil.RunnerAutostart
	installed, serves, target := cmdutil.RunnerInstalled, cmdutil.RunnerServes, cmdutil.RunnerTarget
	t.Cleanup(func() {
		cmdutil.RunnerRunning, cmdutil.RunnerStart, cmdutil.RunnerAutostart = running, start, autostart
		cmdutil.RunnerInstalled, cmdutil.RunnerServes, cmdutil.RunnerTarget = installed, serves, target
	})
	var starts []string
	cmdutil.RunnerRunning = func(string) bool { return false }
	cmdutil.RunnerStart = func(home string) error { starts = append(starts, home); return nil }
	cmdutil.RunnerAutostart = func() string { return "" }
	cmdutil.RunnerInstalled = func() bool { return false }
	cmdutil.RunnerServes = func(string) bool { return false }
	cmdutil.RunnerTarget = func() (string, error) { return "", errors.New("not installed") }
	return &starts
}

func AddProposal(t *testing.T, store core.Store, root string, candidates ...core.Candidate) {
	t.Helper()
	if err := store.Update(func(st *core.State) error {
		st.Proposals = append(st.Proposals, core.Proposal{ID: st.NextID, Kind: core.ProposalLeftovers, Project: root, Candidates: candidates})
		st.NextID++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func DoctorFixture(t testing.TB) (string, cmdutil.Executable) {
	t.Helper()
	IsolateRunner(t)
	root := t.TempDir()
	for key, sub := range map[string]string{"OBIT_HOME": "data", "OBIT_CONFIG_DIR": "config", "OBIT_USER_HOME": "home", "CLAUDE_CONFIG_DIR": "claude", "CODEX_HOME": "codex"} {
		t.Setenv(key, filepath.Join(root, sub))
	}
	exe := filepath.Join(root, "bin", "obit.exe")
	WriteFixture(t, exe, "test executable")
	WriteFixture(t, filepath.Join(root, "bin", "obitd.exe"), "test companion")
	WriteFixture(t, filepath.Join(root, "data", "store.json"), `{"version":2,"nextId":1,"items":[]}`)
	t.Setenv("PATH", filepath.Dir(exe))
	return root, func() (string, error) { return exe, nil }
}

// Compare contents, directory entries, modes and file modification times. This
// catches lock/probe files, missing-directory creation and rewritten configs.
func TreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content := ""
		var modified int64 // Windows can defer directory timestamp updates until a later read.
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content = string(data)
			modified = info.ModTime().UnixNano()
		}
		result[path] = fmt.Sprintf("%s %d %s", info.Mode(), modified, content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
