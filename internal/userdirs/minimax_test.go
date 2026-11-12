package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMiniMaxDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("MINIMAX_DATA_DIR", "")
	t.Setenv("MAVIS_DATA_DIR", "")
	t.Setenv("MCODE_INSTALL_DIR", "")
	if got, err := AgentDir("minimax-code"); err != nil || got != filepath.Join(home, ".minimax") {
		t.Fatalf("default: %q %v", got, err)
	}
	if got, err := MiniMaxProfileDir("work"); err != nil || got != filepath.Join(home, ".minimax-work") {
		t.Fatalf("profile: %q %v", got, err)
	}
	for _, invalid := range []string{"../outside", `a\b`, "bad.", " bad", "a:b", "a\x00b"} {
		if got, err := MiniMaxProfileDir(invalid); err == nil {
			t.Fatalf("unsafe profile %q: %q", invalid, got)
		}
	}
	legacy, primary := filepath.Join(home, "legacy"), filepath.Join(home, "primary")
	t.Setenv("MAVIS_DATA_DIR", legacy)
	if got, err := MiniMaxDir(); err != nil || got != legacy {
		t.Fatalf("legacy override: %q %v", got, err)
	}
	for _, value := range nativeDirectorySpellings([]string{primary, `\\?\` + primary, " " + primary + " "}) {
		t.Setenv("MINIMAX_DATA_DIR", value)
		if got, err := MiniMaxProfileDir("work"); err != nil || got != primary {
			t.Fatalf("primary override: %q %v", got, err)
		}
		if got, err := MiniMaxInstallDir(); err != nil || got != filepath.Join(home, ".minimax-code") {
			t.Fatalf("install root: %q %v", got, err)
		}
	}
	t.Setenv("MCODE_INSTALL_DIR", filepath.Join(home, "binaries"))
	if got, err := MiniMaxInstallDir(); err != nil || got != filepath.Join(home, "binaries") {
		t.Fatalf("install override: %q %v", got, err)
	}
	if got, err := MiniMaxDir(); err != nil || got != primary {
		t.Fatalf("install override changed data root: %q %v", got, err)
	}
	t.Run("windows-path-spelling", func(t *testing.T) {
		windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
		t.Setenv("MINIMAX_DATA_DIR", `\\?\`+primary+`.\child`)
		if got, err := MiniMaxDir(); err == nil {
			t.Fatalf("unsafe override: %q", got)
		}
	})
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("resolver wrote files: %v %v", entries, err)
	}
}
