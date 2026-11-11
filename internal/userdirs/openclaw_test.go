package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenClawLocations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	for _, variable := range []string{"OPENCLAW_HOME", "OPENCLAW_STATE_DIR", "OPENCLAW_CONFIG_PATH", "OPENCLAW_PROFILE"} {
		t.Setenv(variable, "")
	}
	want := filepath.Join(home, ".openclaw")
	if got, err := AgentDir("openclaw"); err != nil || got != want {
		t.Fatalf("default: %q %v", got, err)
	}
	if got, err := OpenClawConfigFile(); err != nil || got != filepath.Join(want, "openclaw.json") {
		t.Fatalf("default config: %q %v", got, err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("resolver wrote directory: %v", err)
	}
	customHome := filepath.Join(home, "custom-home")
	for _, value := range []string{customHome, "~/custom-home", `~\custom-home`} {
		t.Setenv("OPENCLAW_HOME", value)
		if got, err := OpenClawDir(); err != nil || got != filepath.Join(customHome, ".openclaw") {
			t.Fatalf("custom home: %q %v", got, err)
		}
	}
	t.Setenv("OPENCLAW_STATE_DIR", " ~/state ")
	want = filepath.Join(customHome, "state")
	if got, err := OpenClawDir(); err != nil || got != want {
		t.Fatalf("tilde state: %q %v", got, err)
	}
	t.Setenv("OPENCLAW_CONFIG_PATH", `~\config\custom.json5`)
	if got, err := OpenClawConfigFile(); err != nil || got != filepath.Join(customHome, "config", "custom.json5") {
		t.Fatalf("custom config: %q %v", got, err)
	}
	if got, err := AgentDir("openclaw"); err != nil || got != want {
		t.Fatalf("config override changed extensions root: %q %v", got, err)
	}
	t.Setenv("OPENCLAW_PROFILE", "work")
	if got, err := OpenClawDir(); err != nil || got != want {
		t.Fatalf("explicit state dir: %q %v", got, err)
	}
	t.Setenv("OPENCLAW_STATE_DIR", "")
	if got, err := OpenClawDir(); err == nil {
		t.Fatalf("named profile silently used default: %q", got)
	}
}

func TestOpenClawValidatesOverrides(t *testing.T) {
	for _, variable := range []string{"OPENCLAW_HOME", "OPENCLAW_STATE_DIR", "OPENCLAW_CONFIG_PATH"} {
		t.Run(variable, func(t *testing.T) {
			for _, key := range []string{"OPENCLAW_HOME", "OPENCLAW_STATE_DIR", "OPENCLAW_CONFIG_PATH", "OPENCLAW_PROFILE"} {
				t.Setenv(key, "")
			}
			path := filepath.Join(t.TempDir(), "config")
			t.Setenv(variable, `\\?\`+path)
			get, want := OpenClawDir, path
			if variable == "OPENCLAW_HOME" {
				want = filepath.Join(path, ".openclaw")
			}
			if variable == "OPENCLAW_CONFIG_PATH" {
				get = OpenClawConfigFile
			}
			if got, err := get(); err != nil || got != want {
				t.Fatalf("extended: %q %v", got, err)
			}
			for _, invalid := range []string{`\\?\` + path + `.\child`, `\\?\` + path + ` \child`, path + `:stream`} {
				t.Setenv(variable, invalid)
				if got, err := get(); err == nil {
					t.Fatalf("unstable override accepted: %q", got)
				}
			}
		})
	}
}

func TestOpenClawSharedSkillsIsolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("OPENCLAW_HOME", "")
	t.Setenv("OPENCLAW_STATE_DIR", "")
	t.Setenv("OPENCLAW_PROFILE", "")
	got, shared, err := OpenClawSkillsDir()
	if err != nil || !shared || got != filepath.Join(home, ".agents", "skills") {
		t.Fatalf("default: %q %t %v", got, shared, err)
	}
	custom := filepath.Join(home, "redirected")
	t.Setenv("OPENCLAW_HOME", custom)
	got, shared, err = OpenClawSkillsDir()
	if err != nil || !shared || got != filepath.Join(custom, ".agents", "skills") {
		t.Fatalf("home override: %q %t %v", got, shared, err)
	}
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "profile"))
	got, shared, err = OpenClawSkillsDir()
	if err != nil || shared || got != filepath.Join(home, "profile", "skills") {
		t.Fatalf("profile: %q %t %v", got, shared, err)
	}
	t.Setenv("OPENCLAW_STATE_DIR", home+":stream")
	if _, _, err := OpenClawSkillsDir(); err == nil {
		t.Fatal("accepted unstable state")
	}
}
