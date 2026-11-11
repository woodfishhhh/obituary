package testenv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
)

func TestDenyAccessRestoresFixtureACL(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := fixtureDACL(t, path)
	t.Run("denied", func(t *testing.T) {
		testenv.DenyAccess(t, path, windows.FILE_READ_DATA)
		if _, err := os.ReadFile(path); !os.IsPermission(err) {
			t.Fatalf("not denied: %v", err)
		}
	})
	if got := fixtureDACL(t, path); got != before {
		t.Fatalf("ACL not restored: %s => %s", before, got)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("fixture changed: %q %v", data, err)
	}
}
