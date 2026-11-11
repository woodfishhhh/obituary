package pathutil

import "testing"

func TestExplorerMetadata(t *testing.T) {
	for _, name := range []string{"desktop.ini", "DESKTOP.INI", ".obit-labels-x.tmp", ".OBIT-LABELS-active.TMP"} {
		if !IsExplorerMetadata(name) {
			t.Errorf("label metadata not reserved: %q", name)
		}
	}
	for _, name := range []string{"report.log", "desktop.ini.txt", ".obit-labels-x.log", "labels-x.tmp"} {
		if IsExplorerMetadata(name) {
			t.Errorf("ordinary name reserved: %q", name)
		}
	}
}
