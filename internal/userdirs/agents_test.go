package userdirs

import (
	"path/filepath"
	"testing"
)

func TestAgentDirExistingLocations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	for _, tc := range []struct{ name, env, leaf string }{{"claude", "CLAUDE_CONFIG_DIR", ".claude"}, {"codex", "CODEX_HOME", ".codex"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.env, "")
			got, err := AgentDir(tc.name)
			if err != nil || got != filepath.Join(home, tc.leaf) {
				t.Fatalf("default: %q %v", got, err)
			}
			override := filepath.Join(t.TempDir(), "config")
			t.Setenv(tc.env, override)
			if got, err := AgentDir(tc.name); err != nil || got != override {
				t.Fatalf("override: %q %v", got, err)
			}
			t.Setenv(tc.env, `\\?\`+override)
			if got, err := AgentDir(tc.name); err != nil || got != override {
				t.Fatalf("extended spelling: %q %v", got, err)
			}
			t.Setenv(tc.env, `\\?\`+override+".\\child")
			if got, err := AgentDir(tc.name); err == nil {
				t.Fatalf("unstable prefix accepted: %q", got)
			}
		})
	}
	if _, err := AgentDir("unknown"); err == nil {
		t.Fatal("unknown directory accepted")
	}
}

func TestAgentDirectoryRegistration(t *testing.T) {
	for _, dir := range []AgentDirectory{{}, {Name: "missing"}, {Name: "claude", Resolve: ClaudeDir}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid registration accepted: %+v", dir)
				}
			}()
			RegisterAgentDirectory(dir)
		}()
	}
}
