package uninstall

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"errors"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestPOSIXUninstallKeepsDataAndUsesNativeDaemonName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX timer and metadata uninstall without Windows desktop APIs")
	}
	d, root, calls := fixture(t)
	d.runRegistry, d.menuRegistry = nil, nil
	d.previewSchedule = func(exe, _, _ string) string {
		if filepath.Base(exe) != "obitd" {
			t.Errorf("POSIX timer executable: %s", exe)
		}
		return "remove owned user timer"
	}
	keep := filepath.Join(root, "OBIT_HOME", "user-data")
	write(t, keep, "keep")
	before := snapshot(t, root)
	if out, err := execute(t, d, "", "--dry-run"); err != nil || !strings.Contains(out, "remove owned user timer") {
		t.Fatalf("POSIX uninstall preview: %s %v", out, err)
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, before) || len(*calls) != 0 {
		t.Fatal("dry-run changed files or services")
	}
	if out, err := execute(t, d, "", "--yes"); err != nil {
		t.Fatalf("POSIX uninstall: %s %v", out, err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" || len(*calls) != 2 || (*calls)[0] != "schedule" {
		t.Fatalf("uninstall changed data or skipped service cleanup: %q %v %v", data, err, *calls)
	}
}

func TestUninstallSharedOwnersKeepsManualBoundary(t *testing.T) {
	d, root, _ := fixture(t)
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	planner := initcmd.Planner{Executable: d.executable}
	agents := []string{"codex", "hermes", "kimi", "vibe", "qoder"}
	scope := integrate.Scope{Project: root}
	changes, err := planner.IntegrationPlans(agents, scope, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(changes); err != nil {
		t.Fatal(err)
	}
	out, err := execute(t, d, "", "--yes")
	if err == nil || !strings.Contains(err.Error(), "manual agent removal steps remain") {
		t.Fatal(out, err)
	}
	for _, agent := range agents {
		if !strings.Contains(out, agent) {
			t.Fatal("missing installed agent", agent, out)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err == nil && strings.Contains(string(data), "obit:begin") {
		t.Fatal("shared instruction block retained", string(data))
	}
}

func TestReviewScheduleFailureStillRemovesIndependentMenu(t *testing.T) {
	testenv.WindowsOnly(t, "Windows desktop shortcuts, Explorer menu, or Run registration")
	d, _, _ := fixture(t)
	exe, _ := d.executable.Path()
	changes, err := contextmenu.Plan(d.menuRegistry, exe, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := contextmenu.Apply(d.menuRegistry, changes); err != nil {
		t.Fatal(err)
	}
	d.removeSchedule = func() error { return errors.New("task query denied") }
	_, err = execute(t, d, "", "--yes")
	if err == nil || len(d.menuRegistry.(menuRegistry)) != 0 {
		t.Fatalf("independent menus retained after task failure: %v %v", d.menuRegistry, err)
	}
}
