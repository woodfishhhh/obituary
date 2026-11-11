package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Capture the production default before TestMain installs the test substitute.
var defaultFileSync = syncFile

func TestFileSyncDefaultFlushes(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "flush-probe"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := defaultFileSync(file); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("production default skipped file sync: %v", err)
	}
}
