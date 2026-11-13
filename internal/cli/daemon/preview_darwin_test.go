package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
)

func TestPOSIXLaunchdPreviewDoesNotInstall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_USER_HOME", root)
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	self := cmdutil.Executable(func() (string, error) { return filepath.Join(root, "bin", "obit"), nil })
	var out bytes.Buffer
	if err := preview(&out, self, filepath.Join(root, "data"), true, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Library/LaunchAgents", "<integer>300</integer>", "launchctl bootstrap", "preview made no changes"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, out.String())
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("preview changed user files: %v %v", entries, err)
	}
}
