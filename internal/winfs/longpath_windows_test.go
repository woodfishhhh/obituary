package winfs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLongPathRejectsExtendedDeviceNames(t *testing.T) {
	t.Parallel()
	path := `\\?\` + filepath.Join(t.TempDir(), "nul")
	if got, _, err := (FS{}).LongPath(path); err == nil {
		t.Fatalf("extended device name accepted: %q", got)
	}
}

func TestLongPathRejectsUnstableAncestor(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Long Directory Name ", "nul"} {
		for _, missing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/missing=%v", name, missing), func(t *testing.T) {
				t.Parallel()
				parent := filepath.Join(t.TempDir(), name)
				if err := os.Mkdir(`\\?\`+parent, 0700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(parent, "payload.txt")
				if !missing {
					if err := os.WriteFile(`\\?\`+path, []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if got, _, err := (FS{}).LongPath(path); err == nil || !strings.Contains(err.Error(), "use an ID") {
					t.Fatalf("unstable ancestor accepted as an ordinary path: %q %v", got, err)
				}
			})
		}
	}
}

func TestLongPathRejectsExpandedDotWithMissingLeaf(t *testing.T) {
	t.Parallel()
	dotted := `\\?\` + filepath.Join(t.TempDir(), "Long Directory Name.")
	if err := os.Mkdir(dotted, 0700); err != nil {
		t.Fatal(err)
	}
	p, _ := windows.UTF16PtrFromString(dotted)
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(buf[:n])
	if strings.EqualFold(short, dotted) {
		t.Skip("volume does not generate 8.3 aliases")
	}
	// Neither spelling identifies an existing leaf, so the identity check
	// cannot mask the separate normalization check after expansion.
	for _, input := range []string{short + `\absent.txt`, strings.TrimPrefix(short, `\\?\`) + `\absent.txt`} {
		if got, _, err := (FS{}).LongPath(input); err == nil || !strings.Contains(err.Error(), "Windows normalization changes") {
			t.Fatalf("expanded unsafe name escaped normalization check: %q %v", got, err)
		}
	}
}

func TestLongPathExtendedUNC(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "Existing Long Directory")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	unc := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(dir), ":") + "$" + dir[len(filepath.VolumeName(dir)):]
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("existing administrative share unavailable: %v", err)
	}
	for _, suffix := range []string{"", `\missing\payload.txt`} {
		got, complete, err := (FS{}).LongPath(`\\?\UNC\` + unc[2:] + suffix)
		if err != nil || !complete || !strings.EqualFold(got, unc+suffix) {
			t.Fatalf("extended UNC expansion lost share root: %q %v complete=%v", got, err, complete)
		}
	}
}

func TestLongPathTraverseOnlyAncestor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ancestor := filepath.Join(root, "Traversal Only Ancestor")
	path := filepath.Join(ancestor, "Readable Child Directory", "payload.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	inputName, _ := windows.UTF16PtrFromString(path)
	shortBuffer := make([]uint16, 32768)
	shortSize, err := windows.GetShortPathName(inputName, &shortBuffer[0], uint32(len(shortBuffer)))
	if err != nil {
		t.Fatal(err)
	}
	shortInput := windows.UTF16ToString(shortBuffer[:shortSize])
	if !strings.Contains(filepath.Base(filepath.Dir(shortInput)), "~") {
		t.Skip("volume does not provide short-name aliases")
	}
	original, err := windows.GetNamedSecurityInfo(ancestor, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := original.DACL()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	denied, err := windows.SecurityDescriptorFromString("D:P(D;;0x1;;;" + sid + ")(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	denyACL, _, err := denied.DACL()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.SetNamedSecurityInfo(ancestor, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
			t.Error(err)
		}
	})
	if err := windows.SetNamedSecurityInfo(ancestor, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, denyACL, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadDir(ancestor); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("fixture must deny directory listing: %v", err)
	}
	got, complete, err := (FS{}).LongPath(shortInput)
	if err != nil || !complete || !strings.EqualFold(got, path) {
		t.Fatalf("long-name expansion requires directory listing: %s %v (complete=%v)", got, err, complete)
	}
}

func BenchmarkLongPathDepth(b *testing.B) {
	for _, depth := range []int{1, 14, 40} {
		b.Run(fmt.Sprint(depth), func(b *testing.B) {
			path := b.TempDir()
			for range depth {
				path = filepath.Join(path, "Directory")
			}
			if err := os.MkdirAll(path, 0700); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for b.Loop() {
				if _, _, err := (FS{}).LongPath(path); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestLongPathPreservesJunctionAndMissingSuffix(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root := t.TempDir()
	real, link := filepath.Join(root, "Real Directory"), filepath.Join(root, "Linked Directory")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	n, _ := windows.UTF16PtrFromString(link)
	buf := make([]uint16, 32768)
	size, err := windows.GetShortPathName(n, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(buf[:size])
	got, _, err := (FS{}).LongPath(filepath.Join(short, "missing", "payload.txt"))
	if err != nil || !strings.EqualFold(got, filepath.Join(link, "missing", "payload.txt")) {
		t.Fatalf("long path lost lexical junction or suffix: %s %v", got, err)
	}
}

func TestLongPathUNCExistingShare(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	long := filepath.Join(root, "Long Directory Name")
	if err := os.Mkdir(long, 0700); err != nil {
		t.Fatal(err)
	}
	unc := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(long), ":") + "$" + long[len(filepath.VolumeName(long)):]
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("existing administrative share unavailable; no share created: %v", err)
	}
	p, _ := windows.UTF16PtrFromString(unc)
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Skipf("share does not expose short names: %v", err)
	}
	input := filepath.Join(windows.UTF16ToString(buf[:n]), "missing", "file.txt")
	got, complete, err := (FS{}).LongPath(input)
	want := filepath.Join(unc, "missing", "file.txt")
	if err != nil || !complete || !strings.EqualFold(got, want) {
		t.Fatalf("UNC expansion lost lexical share or suffix: %s %v complete=%v", got, err, complete)
	}
}
