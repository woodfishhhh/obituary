//go:build linux

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxAutostartPlanWritesAndRemovesOwnedEntry(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	daemon := "/opt/obituary/obitd"
	home := filepath.Join(t.TempDir(), "data files")
	config := filepath.Join(t.TempDir(), "config files")

	plan, err := PlanAutostart(nil, daemon, home, config, true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Pending() || plan.Before != "" || plan.After == "" {
		t.Fatalf("preview: %+v", plan)
	}
	if _, err := os.Stat(plan.Name); !os.IsNotExist(err) {
		t.Fatalf("preview created %s: %v", plan.Name, err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(plan.Name)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"X-Obituary-Managed=true", "--tray", "\"" + home + "\"", "\"" + config + "\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("desktop entry missing %q: %s", want, text)
		}
	}

	again, err := PlanAutostart(nil, daemon, home, config, true)
	if err != nil || again.Pending() {
		t.Fatalf("idempotence: %+v %v", again, err)
	}
	remove, err := PlanAutostart(nil, daemon, home, config, false)
	if err != nil || !remove.Pending() {
		t.Fatalf("remove preview: %+v %v", remove, err)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plan.Name); !os.IsNotExist(err) {
		t.Fatalf("autostart entry remains: %v", err)
	}
}

func TestLinuxAutostartRejectsForeignEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	autostart := filepath.Join(dir, "autostart")
	if err := os.MkdirAll(autostart, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(autostart, "obituary.desktop")
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nName=User entry\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanAutostart(nil, "/opt/obitd", "/data", "/config", true); err == nil {
		t.Fatal("foreign autostart entry was accepted")
	}
}
