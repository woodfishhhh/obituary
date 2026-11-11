package cmdutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	burycmd "github.com/woodfishhhh/obituary/internal/cli/bury"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	removecmd "github.com/woodfishhhh/obituary/internal/cli/remove"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
)

func TestBurialHelpUsesCurrentConfig(t *testing.T) {
	for _, factory := range []func() *cobra.Command{burycmd.NewCmd, func() *cobra.Command { return removecmd.NewCmd(false) }, func() *cobra.Command { return removecmd.NewCmd(true) }} {
		t.Run(factory().Name(), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("OBIT_CONFIG_DIR", root)
			// Construct before writing config: help must use the invocation's configuration.
			cmd := factory()
			for _, tc := range []struct{ config, want, forbidden string }{
				{"[graveyard]\nmode = 'recycle'\n", "restore from the Recycle Bin", "obit resurrect"},
				{"[graveyard]\nmode = 'graveyard'\n", "restore with obit resurrect", "Recycle Bin"},
				{"[broken", "recovery depends on that mode", "restore with obit resurrect"},
			} {
				clitest.WriteFixture(t, filepath.Join(root, "config.toml"), tc.config)
				out, _, err := clitest.Execute(t, cmd, "--help")
				if err != nil || !strings.Contains(out, tc.want) || strings.Contains(out, tc.forbidden) {
					t.Fatalf("help = %s, %v", out, err)
				}
			}
		})
	}
}

func TestRecycleCommandReceipts(t *testing.T) {
	for _, name := range []string{"bury", "rm"} {
		t.Run(name, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, _ := clitest.CommandDir(t)
			clitest.WriteFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[graveyard]\nmode = 'recycle'\n")
			svc, err := cmdutil.Open()
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "recycled")
			svc.FS = receiptRecycleFS{svc.FS, destination}
			old := cmdutil.Open
			cmdutil.Open = func() (*core.Service, error) { return svc, nil }
			t.Cleanup(func() { cmdutil.Open = old })
			path := filepath.Join(repo, "victim.txt")
			clitest.WriteFixture(t, path, "keep")
			cmd, args := burycmd.NewCmd(), []string{path}
			if name == "rm" {
				cmd, args = removecmd.NewCmd(false), []string{"-v", path}
			}
			out, _, err := clitest.Execute(t, cmd, args...)
			if err != nil || !strings.Contains(out, cmdutil.RecycleRecovery) || strings.Contains(out, "obit resurrect") {
				t.Fatalf("receipt = %s, %v", out, err)
			}
			data, err := os.ReadFile(destination)
			if err != nil || string(data) != "keep" {
				t.Fatal("fake recycle lost data", err)
			}
		})
	}
}

type receiptRecycleFS struct {
	core.FS
	destination string
}

func (f receiptRecycleFS) Recycle(path string, want core.Identity) error {
	return f.MovePinned(path, want, f.destination)
}

func TestBurialSessionPrecedence(t *testing.T) {
	for _, tc := range []struct{ explicit, claude, codex, want string }{
		{" raw session ", "a", "b", " raw session "},
		{"", "a", "b", "claude:a"},
		{"", "", "a", "codex:a"},
		{"", "", "", ""},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Setenv("OBIT_SESSION_ID", tc.explicit)
			t.Setenv("CLAUDE_CODE_SESSION_ID", tc.claude)
			t.Setenv("CODEX_THREAD_ID", tc.codex)
			session, _ := cmdutil.BurialScope()
			if session != tc.want {
				t.Fatalf("session = %q, want %q", session, tc.want)
			}
		})
	}
}

func TestBurialWithoutProjectDiscovery(t *testing.T) {
	for _, command := range []string{"bury", "rm"} {
		t.Run(command, func(t *testing.T) {
			clitest.IsolateRunner(t)
			repo, store := clitest.CommandDir(t)
			t.Setenv("OBIT_SESSION_ID", "explicit-session")
			cwd := `\\?\` + filepath.Join(filepath.Dir(repo), "space ", "child")
			if err := os.MkdirAll(cwd, 0700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwd)
			actual, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if _, found, err := project.Find(actual); err == nil || found {
				t.Fatalf("fixture did not fail project discovery: %q %v", actual, err)
			}
			path := filepath.Join(repo, "target")
			clitest.WriteFixture(t, path, "keep")
			cmd := burycmd.NewCmd()
			if command == "rm" {
				cmd = removecmd.NewCmd(false)
			}
			out, stderr, err := clitest.Execute(t, cmd, path)
			if err != nil {
				t.Fatalf("project metadata blocked a valid deletion: %s %s %v", out, stderr, err)
			}
			st, err := store.ReadSnapshot()
			if err != nil || len(st.Burials) != 1 || st.Burials[0].Operation.Project != "" || st.Burials[0].Operation.Session != "explicit-session" {
				t.Fatalf("wrong best-effort scope: %+v %v", st.Burials, err)
			}
			if data, err := os.ReadFile(st.Burials[0].Grave); err != nil || string(data) != "keep" {
				t.Fatal("burial content changed", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("target was not buried", err)
			}
			cmd = burycmd.NewCmd()
			args := []string{store.Home}
			if command == "rm" {
				cmd = removecmd.NewCmd(false)
				args = append([]string{"-r"}, args...)
			}
			out, stderr, err = clitest.Execute(t, cmd, args...)
			if err == nil || !strings.Contains(out+stderr+err.Error(), "system or Obituary location") {
				t.Fatalf("project discovery failure bypassed the target guard: %s %s %v", out, stderr, err)
			}
			if _, err := os.Stat(store.Home); err != nil {
				t.Fatal("protected data directory changed", err)
			}
		})
	}
}
