package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVibeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	for _, tc := range []struct{ env, want string }{
		{"", filepath.Join(home, ".vibe")},
		{filepath.Join(home, "custom"), filepath.Join(home, "custom")},
		{`\\?\` + filepath.Join(home, "custom"), filepath.Join(home, "custom")},
		{"~", home}, {"~/custom", filepath.Join(home, "custom")}, {`~\custom`, filepath.Join(home, "custom")},
	} {
		t.Setenv("VIBE_HOME", tc.env)
		if got, err := AgentDir("vibe"); err != nil || got != tc.want {
			t.Fatalf("env %q: %q %v", tc.env, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "custom")); !os.IsNotExist(err) {
		t.Fatalf("resolver created directory: %v", err)
	}
	for _, bad := range []string{`\\?\` + home + `.\child`, `\\?\` + home + `\nul\child`, `\\?\` + home + `\file:stream`} {
		t.Setenv("VIBE_HOME", bad)
		if got, err := VibeDir(); err == nil {
			t.Fatalf("unstable override accepted: %q", got)
		}
	}
	t.Setenv("VIBE_HOME", "")
	t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
	if got, err := VibeDir(); err == nil {
		t.Fatalf("unstable default home accepted: %q", got)
	}
}
