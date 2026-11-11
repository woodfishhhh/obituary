package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDroidDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("FACTORY_HOME_OVERRIDE", "")
	got, err := AgentDir("droid")
	if err != nil || got != filepath.Join(home, ".factory") {
		t.Fatalf("default: %q %v", got, err)
	}
	custom := filepath.Join(t.TempDir(), "custom")
	want := filepath.Join(custom, ".factory")
	for _, value := range []string{custom, `\\?\` + custom} {
		t.Setenv("FACTORY_HOME_OVERRIDE", value)
		got, err := DroidDir()
		if err != nil || got != want {
			t.Fatalf("home override: %q %v", got, err)
		}
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatal("resolver created directory", err)
	}
	t.Setenv("FACTORY_HOME_OVERRIDE", `\\?\`+custom+`.\child`)
	if path, err := DroidDir(); err == nil {
		t.Fatal("unstable home accepted", path)
	}
	t.Setenv("FACTORY_HOME_OVERRIDE", custom+":stream")
	if path, err := DroidDir(); err == nil {
		t.Fatal("ADS accepted", path)
	}
}
