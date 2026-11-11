package daemon

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPOSIXDaemonPreviewDoesNotInstallUserUnits(t *testing.T) {
	root := t.TempDir()
	home, config := filepath.Join(root, "data"), filepath.Join(root, "config")
	t.Setenv("OBIT_HOME", home)
	t.Setenv("OBIT_CONFIG_DIR", config)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	self := cmdutil.Executable(func() (string, error) { return filepath.Join(root, "bin", "obit"), nil })
	var out bytes.Buffer
	cmd := NewCmdWithExecutable(self)
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"enable", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"create", "obituary.service", "obituary.timer", "OnCalendar=*:0/5", "systemctl --user enable --now", home, "--config-dir"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, out.String())
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview wrote files: %v %v", entries, err)
	}
	if err := preview(&out, self, home, true, true); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestPOSIXDaemonDisablePreviewPreservesUnits(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	self := cmdutil.Executable(func() (string, error) { return filepath.Join(root, "obit"), nil })
	units, err := app.PreviewSchedule(filepath.Join(root, "obitd"), root, filepath.Join(root, "config"), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range units {
		if err := os.MkdirAll(filepath.Dir(unit.Path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(unit.Path, []byte(unit.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := preview(&out, self, root, false, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "remove") || !strings.Contains(out.String(), "systemctl --user disable --now") {
		t.Fatal(out.String())
	}
	for _, unit := range units {
		data, err := os.ReadFile(unit.Path)
		if err != nil || string(data) != unit.Content {
			t.Fatal("preview changed unit", err)
		}
	}
	if err := os.WriteFile(units[0].Path, []byte("foreign service"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := preview(&out, self, root, true, false); err == nil {
		t.Fatal("preview accepted a foreign unit")
	}
}
