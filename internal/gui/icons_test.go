package gui

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestSettingsReportsDesktopSwitches(t *testing.T) {
	a, _ := fixture(t)
	settings, err := a.Settings()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(settings)
	var fields map[string]any
	_ = json.Unmarshal(data, &fields)
	for _, name := range []string{"trayIcon", "taskbar", "startMenu", "desktopShortcut"} {
		if _, ok := fields[name].(bool); !ok {
			t.Errorf("missing boolean %s: %s", name, data)
		}
	}
}

func TestIconPlansApplyAndPreserveStaleConfig(t *testing.T) {
	a, _ := fixture(t)
	calls := []bool{}
	SetTaskbarSetter(a, func(enabled bool) error { calls = append(calls, enabled); return nil })
	for _, plan := range []func(bool) (Plan, error){a.PlanTrayIcon, a.PlanTaskbar} {
		p, err := plan(false)
		if err != nil || len(p.Changes) != 1 {
			t.Fatal(p, err)
		}
		r, err := a.ApplyPlan(p.ID)
		if err != nil || r.Error != "" {
			t.Fatal(r, err)
		}
	}
	settings, err := a.Settings()
	if err != nil || settings.TrayIcon || settings.Taskbar || len(calls) != 1 || calls[0] {
		t.Fatal(settings, calls, err)
	}
	p, err := a.PlanTaskbar(true)
	if err != nil {
		t.Fatal(err)
	}
	write(t, settings.ConfigPath, "# external edit\n[ui]\ntray=false\ntaskbar=false\n")
	r, err := a.ApplyPlan(p.ID)
	if err != nil || r.Error == "" || len(calls) != 1 {
		t.Fatal("stale config accepted", r, err)
	}
}

func TestShortcutPlansAndSettings(t *testing.T) {
	a, _ := fixture(t)
	for _, plan := range []func(bool) (Plan, error){a.PlanStartMenu, a.PlanDesktopShortcut} {
		p, err := plan(true)
		if err != nil || len(p.Changes) == 0 {
			t.Fatal(p, err)
		}
		for _, change := range p.Changes {
			if change.Target == change.Key {
				t.Fatal("technical target", change)
			}
		}
		if _, err := a.ApplyPlan(p.ID); err != nil {
			t.Fatal(err)
		}
	}
	settings, err := a.Settings()
	if err != nil || !settings.StartMenu || !settings.DesktopShortcut {
		t.Fatal(settings, err)
	}
	for _, plan := range []func(bool) (Plan, error){a.PlanStartMenu, a.PlanDesktopShortcut} {
		p, err := plan(false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.ApplyPlan(p.ID); err != nil {
			t.Fatal(err)
		}
	}
	settings, err = a.Settings()
	if err != nil || settings.StartMenu || settings.DesktopShortcut {
		t.Fatal(settings, err)
	}
}

func TestTaskbarFailureKeepsConfigReceipt(t *testing.T) {
	a, _ := fixture(t)
	SetTaskbarSetter(a, func(bool) error { return errors.New("window unavailable") })
	p, err := a.PlanTaskbar(false)
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.ApplyPlan(p.ID)
	if err != nil || r.Error == "" || len(r.Applied) != 1 {
		t.Fatal(r, err)
	}
	cfg, err := a.readConfig()
	if err != nil || cfg.UI.Taskbar {
		t.Fatal(cfg, err)
	}
	if _, err := os.Stat(r.Applied[0]); err != nil {
		t.Fatal(err)
	}
}
