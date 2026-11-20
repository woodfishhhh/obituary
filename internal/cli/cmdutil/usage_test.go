package cmdutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestUsageIsSilentAndRequiresProductCommand(t *testing.T) {
	testenv.EnableUsage(t)
	root := &cobra.Command{Use: "obit"}
	for _, name := range []string{"doctor", "update", "list"} {
		root.AddCommand(&cobra.Command{Use: name, Run: func(*cobra.Command, []string) {}})
	}
	cmdutil.SetUsage(root)
	for _, name := range []string{"doctor", "update", "list", "list"} {
		out, stderr, err := clitest.Execute(t, root, name)
		if out != "" || stderr != "" || err != nil {
			t.Fatalf("%s: stdout=%q stderr=%q err=%v", name, out, stderr, err)
		}
		_, err = os.Stat(filepath.Join(os.Getenv("OBIT_HOME"), "usage.json"))
		if name == "list" && err != nil || name != "list" && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected report state for %s: %v", name, err)
		}
	}
}
