package gui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Success opens a real Explorer window, so only refusals are tested here.
func TestShowInFolderRefusesWhatIsNotThere(t *testing.T) {
	a := &API{}
	missing := filepath.Join(t.TempDir(), "gone.log")
	if err := a.ShowInFolder(missing); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing item: %v", err)
	}
	file := filepath.Join(t.TempDir(), "kept.log")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.ShowInFolder(file + ":stream"); err == nil {
		t.Fatal("alternate data stream accepted")
	}
}
