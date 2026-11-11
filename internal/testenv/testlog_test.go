package testenv

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTestLogUnknownProtocolKeepsOriginalBytes(t *testing.T) {
	known := "# test log\ngetenv TEMP\ngetenv TEMP\n"
	for _, tc := range []struct {
		name, version, data string
	}{
		{"future-version", "go1.27.0", known},
		{"older-version", "go1.25.4", known},
		{"release-candidate", "go1.26rc1", known},
		{"development-build", "devel go1.26-abcdef", known},
		{"custom-build", "go1.26.4-custom", known},
		{"malformed-version", "go1.26.x", known},
		{"unknown-header", "go1.26.4", "# test log v2\ngetenv TEMP\ngetenv TEMP\n"},
		{"unknown-operation", "go1.26.4", known + "readlink C:\\file\n"},
		{"empty-argument", "go1.26.4", known + "stat \n"},
		{"wrong-separator", "go1.26.4", known + "open\tfile\n"},
		{"relative-cwd", "go1.26.4", known + "chdir relative\n"},
		{"partial-record", "go1.26.4", known + "stat file"},
		{"blank-record", "go1.26.4", known + "\n"},
		{"binary-header", "go1.26.4", "\x00\xff" + known},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "testlog.txt")
			before := []byte(tc.data)
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := compactTestLogFile(path, "", tc.version); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("unknown protocol changed original bytes: %q -> %q (%v)", before, after, err)
			}
		})
	}
}

func TestTestLogKnownProtocolPreservesDependencies(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file with spaces")
	scratch := filepath.Join(root, "deleted-scratch")
	header := "# test log\n"
	deps := "getenv TEMP\nopen " + file + "\nstat " + file + "\n"
	relative := "chdir " + root + "\nopen relative\nopen relative\n"
	data := header + deps + deps + "stat " + filepath.Join(scratch, "removed") + "\n" + relative
	path := filepath.Join(root, "testlog.txt")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := compactTestLogFile(path, scratch, "go1.26.0"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if want := header + deps + relative; err != nil || string(got) != want {
		t.Fatalf("known protocol lost dependencies: got %q, want %q (%v)", got, want, err)
	}
}
