package shortcut

import (
	"github.com/woodfishhhh/obituary/internal/integrate"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-ole/go-ole"
)

func TestNativeUnicodeShortcut(t *testing.T) {
	if os.Getenv("OBIT_TEST_SHORTCUT") != "1" {
		t.Skip("set OBIT_TEST_SHORTCUT=1 for isolated COM shortcuts")
	}
	exe := fixture(t)
	changes, err := Plan(Native{}, exe, Desktop, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := integrate.Apply(changes); err != nil {
		t.Fatal(err)
	}
	path, err := Inspect(Native{}, exe, Desktop)
	if err != nil || path == "" {
		t.Fatal(path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := withLink(data, func(_ *ole.IDispatch, work string) error {
		for _, r := range filepath.Base(work) {
			if r > 127 {
				t.Fatalf("COM saw non-ANSI basename: %s", work)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
