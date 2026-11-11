package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKimiDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("KIMI_SHARE_DIR", "")
	if got, err := AgentDir("kimi"); err != nil || got != filepath.Join(home, ".kimi") {
		t.Fatalf("default: %q %v", got, err)
	}
	override := filepath.Join(home, "custom")
	for _, value := range []string{override, `\\?\` + override} {
		t.Setenv("KIMI_SHARE_DIR", value)
		if got, err := KimiDir(); err != nil || got != override {
			t.Fatalf("override: %q %v", got, err)
		}
		if got, err := KimiSkillsDir(); err != nil || got != filepath.Join(home, ".kimi", "skills") {
			t.Fatalf("skills followed share override: %q %v", got, err)
		}
	}
	if _, err := os.Stat(override); !os.IsNotExist(err) {
		t.Fatalf("resolver created directory: %v", err)
	}
	t.Setenv("KIMI_SHARE_DIR", `\\?\`+override+`.\child`)
	if got, err := KimiDir(); err == nil {
		t.Fatalf("unstable override accepted: %q", got)
	}
	t.Setenv("KIMI_SHARE_DIR", "")
	t.Setenv("OBIT_USER_HOME", `\\?\`+home)
	if got, err := KimiDir(); err != nil || got != filepath.Join(home, ".kimi") {
		t.Fatalf("extended home: %q %v", got, err)
	}
	t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
	if got, err := KimiDir(); err == nil {
		t.Fatalf("unstable home accepted: %q", got)
	}
}
