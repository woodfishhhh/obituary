package shortcut

import (
	"github.com/woodfishhhh/obituary/internal/integrate"
	"os"
	"path/filepath"
	"testing"
)

type textCodec struct{}

func (textCodec) Encode(target string) ([]byte, error) { return []byte(target), nil }
func (textCodec) Target(data []byte) (string, error)   { return string(data), nil }

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "data"))
	t.Setenv("OBIT_USER_HOME", root)
	t.Setenv("OBIT_START_MENU_DIR", filepath.Join(root, "Programs"))
	exe := filepath.Join(root, "obit.exe")
	if err := os.WriteFile(Target(exe), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	return exe
}
func writeLink(t *testing.T, path, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(target), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestShortcutDirectoriesContainOnlyLinks(t *testing.T) {
	for _, place := range []Location{StartMenu, Desktop} {
		t.Run(string(place), func(t *testing.T) {
			exe := fixture(t)
			paths, _ := Paths(place)
			plan, err := Plan(textCodec{}, exe, place, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, change := range plan {
				if filepath.Dir(change.Path) == filepath.Dir(paths[0]) && change.Path != paths[0] {
					t.Errorf("plan puts auxiliary file beside shortcut: %s", change.Path)
				}
			}
			if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
				t.Fatal("planning wrote shortcut", err)
			}
			if err := integrate.Apply(plan); err != nil {
				t.Fatal(err)
			}
			assertShortcutDirectory(t, paths[0], true)
			plan, err = Plan(textCodec{}, exe, place, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := integrate.Apply(plan); err != nil {
				t.Fatal(err)
			}
			assertShortcutDirectory(t, paths[0], false)
		})
	}
}

func assertShortcutDirectory(t *testing.T, path string, installed bool) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	if installed {
		want = 1
	}
	if len(entries) != want || installed && entries[0].Name() != filepath.Base(path) {
		t.Errorf("shortcut directory contains unexpected files: %v", entries)
	}
}

func TestShortcutOwnershipAndUninstall(t *testing.T) {
	exe := fixture(t)
	for _, place := range []Location{StartMenu, Desktop} {
		changes, err := Plan(textCodec{}, exe, place, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := integrate.Apply(changes); err != nil {
			t.Fatal(err)
		}
		path, err := Inspect(textCodec{}, exe, place)
		if err != nil || path == "" {
			t.Fatal(path, err)
		}
		changes, err = Plan(textCodec{}, exe, place, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := integrate.Apply(changes); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("owned shortcut remains", err)
		}
	}
}

func TestShortcutPreservesForeignAndEditedFiles(t *testing.T) {
	exe := fixture(t)
	paths, _ := Paths(Desktop)
	writeLink(t, paths[0], Target(exe))
	changes, err := Plan(textCodec{}, exe, Desktop, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = integrate.Apply(changes)
	if data, err := os.ReadFile(paths[0]); err != nil || string(data) != Target(exe) {
		t.Fatal("unowned shortcut changed", err)
	}
	if plan, err := Plan(textCodec{}, exe, Desktop, true); err != nil || integrate.Pending(plan) {
		t.Fatal("claimed unowned shortcut", err)
	}
}

func TestShortcutExistingStartMenuDoesNotDuplicate(t *testing.T) {
	exe := fixture(t)
	paths, _ := Paths(StartMenu)
	writeLink(t, paths[3], Target(exe))
	changes, err := Plan(textCodec{}, exe, StartMenu, true)
	if err != nil || integrate.Pending(changes) {
		t.Fatal(changes, err)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatal("duplicate created", err)
	}
}

func TestShortcutRejectsChangedPlan(t *testing.T) {
	exe := fixture(t)
	changes, err := Plan(textCodec{}, exe, Desktop, true)
	if err != nil {
		t.Fatal(err)
	}
	paths, _ := Paths(Desktop)
	writeLink(t, paths[0], "user shortcut")
	if err := integrate.Apply(changes); err == nil {
		t.Fatal("overwrote concurrent shortcut")
	}
	data, _ := os.ReadFile(paths[0])
	if string(data) != "user shortcut" {
		t.Fatal("shortcut changed")
	}
}

func TestShortcutEditedOwnedFilePreserved(t *testing.T) {
	exe := fixture(t)
	changes, err := Plan(textCodec{}, exe, Desktop, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(changes); err != nil {
		t.Fatal(err)
	}
	paths, _ := Paths(Desktop)
	writeLink(t, paths[0], "other target")
	changes, err = Plan(textCodec{}, exe, Desktop, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(changes); err == nil {
		t.Fatal("edited shortcut should report preservation")
	}
	if data, _ := os.ReadFile(paths[0]); string(data) != "other target" {
		t.Fatal("edited shortcut removed")
	}
}
