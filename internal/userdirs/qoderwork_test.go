package userdirs

import (
	"path/filepath"
	"testing"
)

func TestQoderWorkDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("QODER_CONFIG_DIR", filepath.Join(home, "qoder"))
	if got, err := AgentDir("qoderwork"); err != nil || got != filepath.Join(home, ".qoderwork") {
		t.Fatalf("default: %q %v", got, err)
	}
	t.Run("windows-path-spelling", func(t *testing.T) {
		windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
		t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
		if _, err := QoderWorkDir(); err == nil {
			t.Fatal("unsafe home accepted")
		}
	})
}
