package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCursorDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	got, err := AgentDir("cursor")
	if err != nil || got != filepath.Join(home, ".cursor") {
		t.Fatalf("default: %q %v", got, err)
	}
	t.Run("windows-path-spelling", func(t *testing.T) {
		windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
		t.Setenv("OBIT_USER_HOME", `\\?\`+home)
		got, err = CursorDir()
		if err != nil || got != filepath.Join(home, ".cursor") {
			t.Fatalf("extended prefix: %q %v", got, err)
		}
	})
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Fatal("resolver created directory", err)
	}
	t.Run("windows-path-spelling", func(t *testing.T) {
		windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
		t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
		if path, err := CursorDir(); err == nil {
			t.Fatal("unstable prefix accepted", path)
		}
	})
}
