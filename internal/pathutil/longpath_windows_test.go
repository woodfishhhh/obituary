package pathutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

func shortName(t *testing.T, path string) string {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n >= uint32(len(buf)) {
		t.Fatalf("short name: %d %v", n, err)
	}
	short := windows.UTF16ToString(buf[:n])
	if strings.EqualFold(short, path) {
		t.Skip("volume does not generate 8.3 aliases")
	}
	return short
}

func TestLongAbsExpandsExistingAncestors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Long Directory Name")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	short := shortName(t, root)
	for _, suffix := range []string{"", `\absent\nested\debug.log`, `\absent\nested\`} {
		for _, prefix := range []string{"", `\\?\`} {
			got, err := pathutil.LongAbs(prefix + short + suffix)
			if err != nil || got != root+suffix {
				t.Errorf("LongAbs(%q) = %q, %v; want %q", prefix+short+suffix, got, err, root+suffix)
			}
		}
	}
	t.Chdir(short)
	got, err := pathutil.LongAbs(`absent\nested\debug.log`)
	if err != nil || got != filepath.Join(root, "absent", "nested", "debug.log") {
		t.Fatalf("short cwd: %q %v", got, err)
	}
}

func TestLongAbsPreservesLexicalJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	root := t.TempDir()
	real, link := filepath.Join(root, "Real Directory"), filepath.Join(root, "Linked Directory")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
		t.Fatalf("junction: %s %v", out, err)
	}
	got, err := pathutil.LongAbs(filepath.Join(shortName(t, link), "absent", "debug.log"))
	if err != nil || got != filepath.Join(link, "absent", "debug.log") {
		t.Fatalf("junction moved during name expansion: %q %v", got, err)
	}
}

func TestLongAbsRejectsUnsafeExpandedAncestor(t *testing.T) {
	root := t.TempDir()
	dotted := `\\?\` + filepath.Join(root, "Long Directory Name.")
	if err := os.Mkdir(dotted, 0700); err != nil {
		t.Fatal(err)
	}
	short := strings.TrimPrefix(shortName(t, dotted), `\\?\`)
	for _, path := range []string{short, filepath.Join(short, "absent", "debug.log"), `\\?\` + short + ".", short + ":stream"} {
		if got, err := pathutil.LongAbs(path); err == nil {
			t.Errorf("unsafe name accepted: %q => %q", path, got)
		}
	}
}
