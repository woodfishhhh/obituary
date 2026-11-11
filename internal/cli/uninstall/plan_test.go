package uninstall

import (
	"errors"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
