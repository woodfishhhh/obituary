package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeepSeekDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	for _, tc := range []struct{ value, want string }{
		{"", filepath.Join(home, ".dsh")}, {"  \t", filepath.Join(home, ".dsh")},
		{"~", home}, {"~/custom", filepath.Join(home, "custom")}, {`~\custom`, filepath.Join(home, "custom")},
		{filepath.Join(home, "override"), filepath.Join(home, "override")},
		{`\\?\` + filepath.Join(home, "override"), filepath.Join(home, "override")},
	} {
		t.Setenv("DSH_HOME", tc.value)
		if got, err := AgentDir("deepseek-harness"); err != nil || got != tc.want {
			t.Fatalf("%q: %q %v", tc.value, got, err)
		}
		if got, err := DeepSeekProfileDir("web"); err != nil || got != filepath.Join(tc.want, "profiles", "web") {
			t.Fatalf("profile: %q %v", got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "override")); !os.IsNotExist(err) {
		t.Fatal("resolver wrote")
	}
	for _, bad := range []string{"", ".", "..", "../web", `a\b`, "C:repo", "web.", "web "} {
		if got, err := DeepSeekProfileDir(bad); err == nil {
			t.Fatalf("invalid profile accepted: %q -> %q", bad, got)
		}
	}
	t.Setenv("DSH_HOME", `\\?\`+home+`.\child`)
	if got, err := DeepSeekDir(); err == nil {
		t.Fatalf("unstable root accepted: %q", got)
	}
}
