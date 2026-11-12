package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLingmaDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("QODER_CONFIG_DIR", filepath.Join(home, "international"))
	t.Setenv("QODERCN_CONFIG_DIR", filepath.Join(home, "cn"))
	got, err := AgentDir("lingma")
	if err != nil || got != filepath.Join(home, ".lingma") {
		t.Fatalf("crossed brand roots: %q %v", got, err)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Fatal("resolver wrote", err)
	}
	t.Run("windows-path-spelling", func(t *testing.T) {
		windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
		t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
		if _, err := LingmaDir(); err == nil {
			t.Fatal("unsafe home accepted")
		}
	})
}
