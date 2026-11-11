package winfs

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestCanonicalReopenErrorClassification(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "file.txt")
	other := filepath.Join(t.TempDir(), "other.txt")
	write(t, path)
	write(t, other)
	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"denied", windows.ERROR_ACCESS_DENIED, windows.ERROR_ACCESS_DENIED},
		{"sharing", windows.ERROR_SHARING_VIOLATION, windows.ERROR_SHARING_VIOLATION},
		{"file-gone", windows.ERROR_FILE_NOT_FOUND, core.ErrUnsafe},
		{"path-gone", windows.ERROR_PATH_NOT_FOUND, core.ErrUnsafe},
		{"replaced", nil, core.ErrUnsafe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got, err := canonical(path, func(p string) (windows.Handle, error) {
				calls++
				if calls == 1 {
					return open(p)
				}
				if tc.err != nil {
					return windows.InvalidHandle, tc.err
				}
				return open(other)
			})
			if got != "" || !errors.Is(err, tc.want) {
				t.Fatalf("Canonical = %q, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestIdentifyNormalizesVolumeSpelling(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "keep.txt")
	write(t, path)
	fs := FS{}
	want, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(path[:1]) + path[1:]
	for _, ref := range []string{lower, `\\?\` + lower} {
		got, err := fs.Identify(ref)
		if err != nil || got != want || got.Volume != strings.ToUpper(filepath.VolumeName(path))+`\` {
			t.Fatalf("Identify(%q) = %+v, %v; want %+v", ref, got, err, want)
		}
		if _, err := fs.Locate(got); err != nil {
			t.Fatalf("normalized identity not locatable: %v", err)
		}
	}
}

func TestLocateReopenErrorClassification(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "keep.txt")
	write(t, path)
	id, err := (FS{}).Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		err    error
		unsafe bool
	}{
		{"gone", core.ErrNotFound, false},
		{"denied", windows.ERROR_ACCESS_DENIED, false},
		{"sharing", windows.ERROR_SHARING_VIOLATION, false},
		{"hardlinks", core.ErrUnsafe, true},
		{"replaced", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := locate(id, func(string) (core.Identity, error) { return core.Identity{}, tc.err })
			if got != "" || err == nil || errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrUnsafe) != tc.unsafe {
				t.Fatalf("Locate = %q, %v; unsafe=%v, object still exists by ID", got, err, tc.unsafe)
			}
		})
	}
}

func TestLocateFollowsARenameAndLosesADeletedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old, moved := filepath.Join(dir, "a.txt"), filepath.Join(dir, "sub", "b.txt")
	write(t, old)
	id, err := FS{}.Identify(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Dir(moved), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(old, moved); err != nil {
		t.Fatal(err)
	}

	found, err := FS{}.Locate(id)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(moved) // long-name form, as the shell reports it
	if !strings.EqualFold(found, want) {
		t.Fatalf("Locate = %s; want %s", found, want)
	}
	if again, _ := (FS{}).Identify(moved); again != id {
		t.Fatalf("identity changed across rename: %+v vs %+v", again, id)
	}

	if err := os.Remove(moved); err != nil {
		t.Fatal(err)
	}
	if _, err := (FS{}).Locate(id); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Locate after delete = %v; want ErrNotFound", err)
	}
}

func TestDirectoriesHaveIdentities(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id, err := FS{}.Identify(dir)
	if err != nil || id.FileID == "" || id.Volume != filepath.VolumeName(dir)+`\` {
		t.Fatalf("Identify(dir) = %+v, %v", id, err)
	}
}

func TestHardLinkedFilesHaveNoUsableIdentity(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	write(t, a)
	if err := os.Link(a, b); err != nil {
		t.Fatal(err)
	}
	if _, err := (FS{}).Identify(b); !errors.Is(err, core.ErrUnsafe) {
		t.Fatalf("Identify(hard link) = %v; want ErrUnsafe", err)
	}
}

func TestCanonicalResolvesLinkedParents(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	dir := t.TempDir()
	real := filepath.Join(dir, "real", "child")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, filepath.Join(dir, "real")).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	got, err := Canonical(filepath.Join(alias, "child"))
	want, _ := filepath.EvalSymlinks(real)
	if err != nil || !strings.EqualFold(got, want) {
		t.Fatalf("Canonical = %q, %v; want %q", got, err, want)
	}
}

func TestCanonicalAcceptsHardLinksAndPreservesJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	write(t, a)
	if err := os.Link(a, b); err != nil {
		t.Fatal(err)
	}
	real, alias := filepath.Join(root, "real"), filepath.Join(root, "alias")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, real).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	for _, path := range []string{a, b, real, alias} {
		got, err := Canonical(path)
		longRoot, _ := filepath.EvalSymlinks(root)
		want := filepath.Join(longRoot, filepath.Base(path))
		if err != nil || !strings.EqualFold(got, want) {
			t.Errorf("Canonical(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
}
