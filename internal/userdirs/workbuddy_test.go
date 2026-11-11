package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkBuddyDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	want := filepath.Join(home, ".workbuddy")
	for _, resolve := range []func() (string, error){WorkBuddyDir, func() (string, error) { return AgentDir("workbuddy") }} {
		got, err := resolve()
		if err != nil || got != want {
			t.Fatalf("directory = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("resolver created directory: %v", err)
	}
	t.Setenv("OBIT_USER_HOME", `\\?\`+home)
	if got, err := WorkBuddyDir(); err != nil || got != want {
		t.Fatalf("extended home = %q, %v", got, err)
	}
	other := t.TempDir()
	t.Setenv("OBIT_USER_HOME", other)
	if got, err := WorkBuddyDir(); err != nil || got != filepath.Join(other, ".workbuddy") {
		t.Fatalf("redirected home = %q, %v", got, err)
	}
	for _, unsafe := range []string{`\\?\` + home + `.\child`, home + `:stream`, `\\?\` + filepath.Join(home, "trailing.", "child")} {
		t.Setenv("OBIT_USER_HOME", unsafe)
		if got, err := WorkBuddyDir(); err == nil || got != "" {
			t.Fatalf("unsafe home accepted: %q, %v", got, err)
		}
	}
}

func TestWorkBuddyInternationalDirectoryRequiresVerification(t *testing.T) {
	t.Parallel()
	if path, err := AgentDir("workbuddy-international"); err == nil {
		t.Fatalf("unverified international MCP directory resolved: %q", path)
	}
}
