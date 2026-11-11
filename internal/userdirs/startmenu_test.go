//go:build windows

package userdirs

import (
	"path/filepath"
	"testing"
)

func TestProgramsRedirected(t *testing.T) {
	menu := t.TempDir()
	t.Setenv("OBIT_START_MENU_DIR", menu)
	got, err := Programs()
	if err != nil || got != menu {
		t.Fatal(got, err)
	}
	t.Setenv("OBIT_START_MENU_DIR", "")
	t.Setenv("OBIT_USER_HOME", menu)
	got, err = Programs()
	if err != nil || got != filepath.Join(menu, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs") {
		t.Fatal(got, err)
	}
}
