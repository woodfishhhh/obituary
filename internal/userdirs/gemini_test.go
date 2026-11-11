package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("GEMINI_CLI_HOME", "")
	got, err := AgentDir("gemini")
	if err != nil || got != filepath.Join(home, ".gemini") {
		t.Fatalf("default: %q %v", got, err)
	}
	custom := filepath.Join(t.TempDir(), "custom")
	want := filepath.Join(custom, ".gemini")
	for _, value := range []string{custom, `\\?\` + custom} {
		t.Setenv("GEMINI_CLI_HOME", value)
		got, err := GeminiDir()
		if err != nil || got != want {
			t.Fatalf("home override: %q %v", got, err)
		}
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatal("resolver created directory", err)
	}
	t.Setenv("GEMINI_CLI_HOME", `\\?\`+custom+`.\child`)
	if path, err := GeminiDir(); err == nil {
		t.Fatal("unstable home accepted", path)
	}
	t.Setenv("GEMINI_CLI_HOME", custom+":stream")
	if path, err := GeminiDir(); err == nil {
		t.Fatal("ADS accepted", path)
	}
}

func TestGeminiSharedSkillsDirectory(t *testing.T) {
	home, custom := t.TempDir(), t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	for _, override := range []string{"", custom} {
		t.Setenv("GEMINI_CLI_HOME", override)
		base := home
		if override != "" {
			base = custom
		}
		got, err := GeminiSkillsDir()
		if err != nil || got != filepath.Join(base, ".agents", "skills") {
			t.Fatalf("shared home: %q %v", got, err)
		}
	}
	t.Setenv("GEMINI_CLI_HOME", custom+":stream")
	if _, err := GeminiSkillsDir(); err == nil {
		t.Fatal("accepted unstable home")
	}
}
