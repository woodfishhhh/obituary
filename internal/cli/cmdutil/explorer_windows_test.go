//go:build windows

package cmdutil_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
)

func TestSkipExplorerCheckKeepsCobrasPromptOnlyForExplorer(t *testing.T) {
	prompt := cobra.MousetrapHelpText
	t.Cleanup(func() { cobra.MousetrapHelpText = prompt })
	cmdutil.SkipExplorerCheck()
	// The parent here is the go tool or a shell, never Explorer.
	if cobra.MousetrapHelpText != "" {
		t.Fatal("a parent that is not Explorer must switch Cobra's process-list check off")
	}
}

// helperVerdict is the child half of TestSkipExplorerCheckParentImage: it reports
// whether SkipExplorerCheck left Cobra's prompt (and so its own check) in place.
func TestExplorerCheckChild(t *testing.T) {
	if os.Getenv("OBIT_TEST_EXPLORER_CHILD") != "1" {
		t.Skip("helper process")
	}
	cmdutil.SkipExplorerCheck()
	fmt.Println("verdict:", map[bool]string{true: "kept", false: "skipped"}[cobra.MousetrapHelpText != ""])
}

// TestExplorerCheckLauncher runs as the parent named explorer.exe and starts the child.
func TestExplorerCheckLauncher(t *testing.T) {
	if os.Getenv("OBIT_TEST_EXPLORER_LAUNCH") != "1" {
		t.Skip("helper process")
	}
	child := exec.Command(os.Args[0], "-test.run=^TestExplorerCheckChild$", "-test.v")
	child.Env = append(os.Environ(), "OBIT_TEST_EXPLORER_CHILD=1", "OBIT_TEST_EXPLORER_LAUNCH=")
	out, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	_, _ = os.Stdout.Write(out)
}

func TestSkipExplorerCheckParentImage(t *testing.T) {
	if testing.Short() {
		t.Skip("starts processes; TestSkipExplorerCheckKeepsCobrasPromptOnlyForExplorer covers the decision in -short")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"explorer.exe": "kept", "Explorer.EXE": "kept", "explorer.exe.bak.exe": "skipped", "notexplorer.exe": "skipped"} {
		t.Run(name, func(t *testing.T) {
			launcher := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(launcher, data, 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(launcher, "-test.run=^TestExplorerCheckLauncher$", "-test.v")
			cmd.Env = append(os.Environ(), "OBIT_TEST_EXPLORER_LAUNCH=1")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("launcher: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), "verdict: "+want) {
				t.Fatalf("a parent named %s must give %q:\n%s", name, want, out)
			}
		})
	}
}
