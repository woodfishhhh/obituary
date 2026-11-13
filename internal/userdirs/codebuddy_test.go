package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodeBuddyDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("CODEBUDDY_CONFIG_DIR", "")
	for _, name := range []string{"codebuddy", "codebuddy-ide"} {
		got, err := AgentDir(name)
		if err != nil || got != filepath.Join(home, ".codebuddy") {
			t.Fatalf("%s: %q %v", name, got, err)
		}
	}
	custom := filepath.Join(t.TempDir(), "custom")
	for _, override := range nativeDirectorySpellings([]string{custom, `\\?\` + custom}) {
		t.Setenv("CODEBUDDY_CONFIG_DIR", override)
		got, err := CodeBuddyDir()
		if err != nil || got != custom {
			t.Fatalf("CLI override: %q %v", got, err)
		}
		ide, err := CodeBuddyIDEDir()
		if err != nil || ide != filepath.Join(home, ".codebuddy") {
			t.Fatalf("undocumented IDE override: %q %v", ide, err)
		}
	}
	legacy, err := CodeBuddyLegacyMCP()
	if err != nil || legacy != filepath.Join(home, ".codebuddy.json") {
		t.Fatalf("legacy: %q %v", legacy, err)
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatal("resolver wrote a directory", err)
	}
	for _, value := range nativeDirectorySpellings([]string{custom + ":stream", `\\?\` + custom + `.\child`}) {
		t.Setenv("CODEBUDDY_CONFIG_DIR", value)
		if got, err := CodeBuddyDir(); err == nil {
			t.Fatal("unsafe override accepted", got)
		}
	}
}
