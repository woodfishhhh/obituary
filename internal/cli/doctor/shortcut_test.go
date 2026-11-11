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

func TestDesktopShortcutDiagnosis(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	exe := filepath.Join(home, "obit.exe")
	for _, installed := range []bool{false, true} {
		check := diagnoseDesktop(exe, func(path string) (string, error) {
			if path != filepath.Join(home, "Desktop", "讣告 Obituary.lnk") {
				t.Fatal("escaped test desktop", path)
			}
			if !installed {
				return "", os.ErrNotExist
			}
			return filepath.Join(home, "obit-gui.exe"), nil
		})
		want := "WARN"
		if installed {
			want = "PASS"
		}
		if check.Name != "desktop" || check.Status != want {
			t.Fatal(check)
		}
	}
}
