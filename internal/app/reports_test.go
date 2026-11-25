package app

import (
	"os"
	"path/filepath"
	"testing"
)

// Reports may get a lifespan inside the otherwise protected data directory.
func TestGuardAllowsReportsInsideTheDataDirectory(t *testing.T) {
	home := filepath.Join(t.TempDir(), "data")
	reports := ReportsDir(home)
	if err := os.MkdirAll(reports, 0o700); err != nil {
		t.Fatal(err)
	}
	report, ledger := filepath.Join(reports, "doctor.html"), filepath.Join(home, "store.json")
	for _, path := range []string{report, ledger} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	g := Guard(home)
	if err := g.Check(report, false); err != nil {
		t.Errorf("report refused: %v", err)
	}
	for _, path := range []string{ledger, reports, home} {
		if err := g.Check(path, true); err == nil {
			t.Errorf("%s accepted", path)
		}
	}
}
