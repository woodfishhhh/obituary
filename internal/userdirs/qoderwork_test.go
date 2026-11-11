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
	t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
	if _, err := QoderWorkDir(); err == nil {
		t.Fatal("unsafe home accepted")
	}
}
