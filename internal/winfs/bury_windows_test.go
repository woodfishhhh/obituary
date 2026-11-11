package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func TestRealPathNeverAliasesTrailingDotDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plain := filepath.Join(root, "logs")
	dotted := `\\?\` + filepath.Join(root, "logs.")
	for _, dir := range []string{plain, dotted} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, "same.txt"))
	}
	for _, path := range []string{dotted, filepath.Join(dotted, "same.txt")} {
		want, err := (FS{}).Identify(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := (FS{}).RealPath(path)
		if errors.Is(err, corefs.ErrUnsafe) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		back, err := (FS{}).Identify(got)
		if err != nil || back.Serial != want.Serial || back.FileID != want.FileID {
			t.Errorf("RealPath(%q) = %q addresses a different object: %+v, %v; want %+v", path, got, back, err, want)
		}
	}
}

func TestMovePinnedMovesTheCheckedObjectAndRefusesASwap(t *testing.T) {
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
	graveDir := filepath.Join(dir, "graveyard", "1-x")
	if err := (FS{}).MakeGrave(graveDir); err != nil {
		t.Fatal(err)
	}
	if err := (FS{}).MakeGrave(graveDir); !errors.Is(err, os.ErrExist) {
		t.Fatalf("MakeGrave must refuse an existing directory, got %v", err)
	}
	grave := filepath.Join(graveDir, "a.txt")
	if err := (FS{}).MovePinned(path, id, grave); err == nil || errors.Is(err, corefs.ErrUnsafe) || errors.Is(err, corefs.ErrNotFound) {
		t.Fatalf("MovePinned(swapped) = %v; want a retryable identity mismatch", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the swapped-in file must stay where it is")
	}

	id, _ = FS{}.Identify(path)
	if err := (FS{}).MovePinned(path, id, grave); err != nil {
		t.Fatal(err)
	}
	if moved, err := (FS{}).Identify(grave); err != nil || moved != id {
		t.Fatalf("grave holds %+v, %v; want the same object %+v", moved, err, id)
	}
}

func TestMovePinnedNeverReplaces(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	from, to := filepath.Join(dir, "from.txt"), filepath.Join(dir, "to.txt")
	write(t, from)
	if err := os.WriteFile(to, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := FS{}.Identify(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := (FS{}).MovePinned(from, id, to); err == nil {
		t.Fatal("MovePinned must not replace an existing file")
	}
	if data, _ := os.ReadFile(to); string(data) != "keep" {
		t.Fatal("the existing file was overwritten")
	}
}
