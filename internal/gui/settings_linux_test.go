//go:build linux

package gui

import "testing"

func TestLinuxSettingsExposeOnlyPortableEntryCapabilities(t *testing.T) {
	a, _ := fixture(t)
	settings, err := a.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Capabilities.ContextMenu || settings.Capabilities.Taskbar || settings.Capabilities.StartMenu || settings.Capabilities.DesktopShortcut {
		t.Fatalf("Windows-only capabilities exposed on Linux: %+v", settings.Capabilities)
	}
}
