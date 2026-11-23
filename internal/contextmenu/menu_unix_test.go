//go:build linux || darwin

package contextmenu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestUserMenuInstallAndRemove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	reg := UserRegistry()
	exe := filepath.Join(home, "bin", "obit")
	changes, err := PlanLanguage(reg, exe, false, i18n.English)
	if err != nil || len(changes) == 0 {
		t.Fatalf("install: %v (%d changes)", err, len(changes))
	}
	if err := Apply(reg, changes); err != nil {
		t.Fatal(err)
	}
	exe = filepath.Join(home, "moved", "obit")
	changes, err = PlanLanguage(reg, exe, false, i18n.Chinese)
	if err != nil || len(changes) == 0 {
		t.Fatalf("relocate and localize: %v", err)
	}
	if err := Apply(reg, changes); err != nil {
		t.Fatal(err)
	}
	installed, err := nativeMenus(exe, i18n.Chinese)
	if err != nil {
		t.Fatal(err)
	}
	var modified string
	for path := range installed {
		modified = path
		break
	}
	if err := os.WriteFile(modified, []byte("mine"), 0600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(filepath.Dir(changes[0].Path), "user-note")
	if err := os.WriteFile(foreign, []byte("mine"), 0600); err != nil {
		t.Fatal(err)
	}
	changes, err = PlanLanguage(reg, exe, true, i18n.English)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(reg, changes); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(foreign); err != nil || string(got) != "mine" {
		t.Fatalf("user file: %q %v", got, err)
	}
	if got, err := os.ReadFile(modified); err != nil || string(got) != "mine" {
		t.Fatalf("edited menu lost: %q %v", got, err)
	}
}
