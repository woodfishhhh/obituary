package userdirs

import (
	"path/filepath"
	"testing"
)

func TestDesktopUsesRedirectedHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	got, err := Desktop()
	if err != nil || got != filepath.Join(home, "Desktop") {
		t.Fatal(got, err)
	}
}
