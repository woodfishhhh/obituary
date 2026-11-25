//go:build linux && desktop

package gui

import (
	"errors"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestLinuxSettingsExposeOnlyPortableEntryCapabilities(t *testing.T) {
	a, _ := fixture(t)
	t.Setenv("OBIT_START_MENU_DIR", "")
	settings, err := a.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Capabilities != (Capabilities{Recycle: true, ContextMenu: true}) {
		t.Fatalf("Windows-only capabilities exposed on Linux: %+v", settings.Capabilities)
	}
	for _, plan := range []func(bool) (Plan, error){a.PlanTaskbar, a.PlanStartMenu, a.PlanDesktopShortcut, a.PlanTrayIcon} {
		if _, err := plan(true); !errors.Is(err, core.ErrUnsupported) {
			t.Fatalf("unsupported desktop operation: %v", err)
		}
	}
	if _, err := a.PlanSettings("recycle", 7); err != nil {
		t.Fatalf("recycle mode: %v", err)
	}
}
