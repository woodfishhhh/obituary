package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopilotDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("COPILOT_HOME", "")
	got, err := AgentDir("copilot")
	if err != nil || got != filepath.Join(home, ".copilot") {
		t.Fatalf("default: %q %v", got, err)
	}
	custom := filepath.Join(t.TempDir(), "custom")
	for _, value := range []string{custom, `\\?\` + custom} {
		t.Setenv("COPILOT_HOME", value)
		got, err := CopilotDir()
		if err != nil || got != custom {
			t.Fatalf("override: %q %v", got, err)
		}
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatal("resolver created directory", err)
	}
	t.Setenv("COPILOT_HOME", `\\?\`+custom+`.\child`)
	if path, err := CopilotDir(); err == nil {
		t.Fatal("unstable prefix accepted", path)
	}
	t.Setenv("COPILOT_HOME", custom+":stream")
	if path, err := CopilotDir(); err == nil {
		t.Fatal("ADS accepted", path)
	}
}
