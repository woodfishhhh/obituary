package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/shortcut"
)

func TestPOSIXDesktopDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native POSIX desktop diagnostics")
	}
	target := func(string) (string, error) {
		t.Fatal("POSIX diagnosis tried to inspect a Windows shortcut")
		return "", nil
	}
	checks := append(diagnoseIcons(), diagnoseShortcut("/tmp/obit", target), diagnoseDesktop("/tmp/obit", target))
	for _, check := range checks {
		if check.Status != "INFO" {
			t.Fatalf("Windows-only feature reported as an installation failure: %+v", check)
		}
	}
}

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

func TestShortcutDiagnosisUsesRelocatedAndLegacyReceipts(t *testing.T) {
	for _, place := range []shortcut.Location{shortcut.StartMenu, shortcut.Desktop} {
		t.Run(string(place), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("OBIT_HOME", filepath.Join(root, "data"))
			t.Setenv("OBIT_USER_HOME", root)
			t.Setenv("OBIT_START_MENU_DIR", filepath.Join(root, "Programs"))
			exe := filepath.Join(root, "obit.exe")
			clitest.WriteFixture(t, shortcut.Target(exe), "fixture")
			plan, err := shortcut.Plan(clitest.ShortcutCodec{}, exe, place, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := integrate.Apply(plan); err != nil {
				t.Fatal(err)
			}
			paths, _ := shortcut.Paths(place)
			receipt, _ := shortcut.ReceiptPath(paths[0])
			target := func(path string) (string, error) { data, err := os.ReadFile(path); return string(data), err }
			for _, legacy := range []bool{false, true} {
				if legacy {
					old := integrate.ManagedAssetReceiptPath(paths[0])
					if err := os.Rename(receipt, old); err != nil {
						t.Fatal(err)
					}
					receipt = old
				}
				before := clitest.TreeSnapshot(t, root)
				check := diagnoseShortcutAt(exe, place, target)
				if check.Status != "PASS" || !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
					t.Fatal("diagnosis failed or changed files", check)
				}
			}
			clitest.WriteFixture(t, receipt, "edited receipt")
			if check := diagnoseShortcutAt(exe, place, target); check.Status != "WARN" {
				t.Fatal("edited receipt accepted", check)
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
