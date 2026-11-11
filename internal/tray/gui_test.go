//go:build windows

package tray

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestGUIMenuUsesSiblingExecutableAndInjectedLauncher(t *testing.T) {
	root := t.TempDir()
	gui := filepath.Join(root, "obit-gui.exe")
	if err := os.WriteFile(gui, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var launched string
	action := newGUIAction(filepath.Join(root, "obitd.exe"), func(path string) error { launched = path; return nil })
	action.localize(i18n.Chinese)
	if !action.enabled || action.title != "打开讣告" {
		t.Fatalf("menu: %+v", action)
	}
	if err := action.open(); err != nil || launched != gui {
		t.Fatalf("launch=%s err=%v", launched, err)
	}
	want := errors.New("cannot launch")
	action.start = func(string) error { return want }
	if !errors.Is(action.open(), want) {
		t.Fatal("launcher failure hidden")
	}
}

func TestGUIMenuDisabledWhenExecutableMissing(t *testing.T) {
	called := false
	action := newGUIAction(filepath.Join(t.TempDir(), "obitd.exe"), func(string) error { called = true; return nil })
	action.localize(i18n.English)
	if action.enabled || !strings.Contains(action.tooltip, "obit-gui.exe") {
		t.Fatalf("missing binary menu: %+v", action)
	}
	if err := action.open(); err == nil || called {
		t.Fatalf("disabled action launched: called=%v err=%v", called, err)
	}
}
