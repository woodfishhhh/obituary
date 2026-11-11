package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAntigravityDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	want := filepath.Join(home, ".gemini", "config")
	if got, err := AgentDir("antigravity"); err != nil || got != want {
		t.Fatalf("default: %q %v", got, err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("resolver created directory: %v", err)
	}
	t.Setenv("OBIT_USER_HOME", `\\?\`+home)
	if got, err := AntigravityDir(); err != nil || got != want {
		t.Fatalf("extended home: %q %v", got, err)
	}
	t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
	if got, err := AntigravityDir(); err == nil {
		t.Fatalf("unstable home accepted: %q", got)
	}
}
