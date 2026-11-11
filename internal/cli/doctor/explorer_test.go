//go:build windows

package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
)

func TestDoctorExplorerScope(t *testing.T) {
	for _, tc := range []struct {
		name, config, scope        string
		enabled, everywhere, inGit bool
	}{
		{"default", "", "Downloads, Desktop and watched folders", true, false, false},
		{"disabled", "[explorer]\nenabled = false\n", "enabled=false", false, false, false},
		{"everywhere", "[explorer]\neverywhere = true\nin_git = true\n", "all tracked locations", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
			clitest.WriteFixture(t, filepath.Join(root, "config", "config.toml"), tc.config)
			before := clitest.TreeSnapshot(t, root)
			got := diagnoseExplorer(filepath.Join(root, "missing"))
			if got.Status != "PASS" || !strings.Contains(got.Detail, tc.scope) || got.Explorer.Enabled != tc.enabled || got.Explorer.Everywhere != tc.everywhere || got.Explorer.InGit != tc.inGit {
				t.Fatalf("Explorer scope: %+v %+v", got, got.Explorer)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("Explorer diagnosis created data or config")
			}
		})
	}
}

func TestDoctorExplorerRetainedFailures(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	clitest.WriteFixture(t, filepath.Join(root, "explorer.json"), `{"a":{"path":"C:\\a"},"b":{"path":"C:\\b"},"healthy":{"path":"C:\\healthy"}}`)
	for _, tc := range []struct {
		name, log, status string
		failures          int
	}{
		{"healthy retained labels", "", "PASS", 0},
		{"one failure", "2026/11/10 12:00:00 Explorer labels: C:\\a: denied\n", "PASS", 0},
		{"unretained target", "C:\\gone: denied\nC:\\gone: denied\nC:\\ab: denied\nC:\\ab: denied\n", "PASS", 0},
		{"repeated joined errors", "2026/11/10 12:00:00 Explorer labels: C:\\a: denied\nC:\\b: changed\n2026/11/10 12:05:00 Explorer labels: C:\\a: denied\nC:\\b: changed\n", "WARN", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clitest.WriteFixture(t, filepath.Join(root, "explorer.log"), tc.log)
			before := clitest.TreeSnapshot(t, root)
			got := diagnoseExplorer(root)
			if got.Status != tc.status || got.Explorer.CleanupTargets != 3 || got.Explorer.RepeatedFailures != tc.failures {
				t.Fatalf("Explorer cleanup failures: %+v %+v", got, got.Explorer)
			}
			if tc.failures > 0 && (got.Explorer.FirstFailure != `C:\a` || !strings.Contains(got.Detail, `first: C:\a`) || !strings.Contains(got.Detail, "desktop.ini")) {
				t.Fatalf("missing cleanup advice: %+v", got)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("Explorer diagnosis changed cleanup evidence")
			}
		})
	}
}

func TestDoctorExplorerInvalidInput(t *testing.T) {
	for _, name := range []string{"config", "index", "log"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("OBIT_CONFIG_DIR", root)
			switch name {
			case "config":
				clitest.WriteFixture(t, filepath.Join(root, "config.toml"), "[explorer")
			case "index":
				clitest.WriteFixture(t, filepath.Join(root, "explorer.json"), "{")
			case "log":
				if err := os.Mkdir(filepath.Join(root, "explorer.log"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			before := clitest.TreeSnapshot(t, root)
			if got := diagnoseExplorer(root); got.Status != "FAIL" {
				t.Fatal(got)
			}
			if !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("Explorer diagnosis repaired damaged input")
			}
		})
	}
}
