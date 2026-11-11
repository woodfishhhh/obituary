//go:build linux || darwin

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPOSIXGuardProtectsSystemAndHome(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user")
	if err := os.Mkdir(user, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_USER_HOME", user)
	g := Guard(filepath.Join(dir, "data"))
	for _, path := range []string{"/", "/etc", user} {
		if err := g.Check(path, true); err == nil {
			t.Fatalf("protected directory accepted: %s", path)
		}
	}
	path := filepath.Join(dir, "scratch")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.Check(path, false); err != nil {
		t.Fatal(err)
	}
}
