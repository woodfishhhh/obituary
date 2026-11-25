package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func TestRecycleRefusesReservedFileBeforeShell(t *testing.T) {
	path := `\\?\` + filepath.Join(t.TempDir(), "NUL")
	write(t, path)
	t.Cleanup(func() { _ = os.Remove(path) })
	// An empty identity prevents even a broken implementation from calling Shell.
	err := (FS{}).Recycle(path, corefs.Identity{})
	if !errors.Is(err, corefs.ErrUnsafe) || !strings.Contains(err.Error(), "graveyard") {
		t.Fatalf("reserved Shell path: %v", err)
	}
}

// TestRecycleMovesToRecycleBin touches the real Recycle Bin, so it only runs
// with OBIT_TEST_RECYCLE=1.
func TestRecycleMovesToRecycleBin(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the real Windows Recycle Bin")
	}
	if os.Getenv("OBIT_TEST_RECYCLE") != "1" {
		t.Skip("set OBIT_TEST_RECYCLE=1 to exercise the real Recycle Bin")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "obit-recycle-test.txt")
	write(t, file)
	folder := filepath.Join(dir, "obit-recycle-test-dir")
	if err := os.MkdirAll(filepath.Join(folder, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(folder, "nested", "f.txt"))

	for _, path := range []string{file, folder} {
		id, err := FS{}.Identify(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := (FS{}).Recycle(path, id); err != nil {
			t.Fatalf("Recycle(%s): %v", path, err)
		}
		if _, err := os.Lstat(path); err == nil {
			t.Fatalf("%s still exists", path)
		}
	}
	if err := (FS{}).Recycle(file, corefs.Identity{}); err == nil {
		t.Fatal("recycling a missing path should fail")
	}
}

func TestRecycleRefusesAnObjectThatWasSwappedIn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	write(t, path)
	id, err := FS{}.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	write(t, path) // same name, different object

	if err := (FS{}).Recycle(path, id); !errors.Is(err, corefs.ErrUnsafe) {
		t.Fatalf("Recycle = %v; want ErrUnsafe", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the swapped-in file must be kept")
	}
}

func TestRecycleRejectsActualDirectory(t *testing.T) {
	dir := t.TempDir()
	backend := FS{}
	id, err := backend.Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = backend.Recycle(dir, id); err == nil || !strings.Contains(err.Error(), "directories require") {
		t.Fatalf("directory recycle gate: %v", err)
	}
}
