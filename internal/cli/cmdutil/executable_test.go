package cmdutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookCommandPinsExecutableOnPath(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "obit.exe")
	if err := os.WriteFile(exe, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(exe))
	self := Executable(func() (string, error) { return exe, nil })
	command := self.HookCommand("codex")
	if !strings.Contains(command, filepath.ToSlash(filepath.Dir(exe))) || command == "obit hook codex" {
		t.Fatalf("hook executable is not pinned: %q", command)
	}
	if command := (Executable(func() (string, error) { return "", errors.New("unavailable") })).HookCommand("codex"); command != "" {
		t.Fatalf("unresolved executable used PATH lookup: %q", command)
	}
}
