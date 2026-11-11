package userdirs

import (
	"path/filepath"
	"testing"
)

func TestRooDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	got, err := AgentDir("roo")
	if err != nil || got != filepath.Join(home, ".roo") {
		t.Fatalf("RooDir = %q, %v", got, err)
	}
}
