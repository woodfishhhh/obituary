package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartMenuShortcutDiagnosis(t *testing.T) {
	menu := t.TempDir()
	t.Setenv("OBIT_START_MENU_DIR", menu)
	exe := filepath.Join(t.TempDir(), "obit.exe")
	for _, tc := range []struct{ name, target, status string }{
		{"absent", "", "WARN"},
		{"current", filepath.Join(filepath.Dir(exe), "obit-gui.exe"), "PASS"},
		{"different", filepath.Join(menu, "other.exe"), "WARN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := diagnoseShortcut(exe, func(path string) (string, error) {
				if !strings.HasPrefix(path, menu) {
					t.Fatal("diagnostic escaped Programs", path)
				}
				if tc.target == "" {
					return "", os.ErrNotExist
				}
				return tc.target, nil
			})
			if check.Status != tc.status {
				t.Fatal(check)
			}
		})
	}
}
