package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPiDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "")
	want := filepath.Join(home, ".pi", "agent")
	if got, err := AgentDir("pi"); err != nil || got != want {
		t.Fatalf("default: %q %v", got, err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("resolver wrote directory: %v", err)
	}
	for _, override := range []string{filepath.Join(home, "custom"), `\\?\` + filepath.Join(home, "custom"), "~/custom", `~\custom`} {
		t.Setenv("PI_CODING_AGENT_DIR", override)
		if got, err := PiDir(); err != nil || got != filepath.Join(home, "custom") {
			t.Fatalf("override %q: %q %v", override, got, err)
		}
	}
	t.Setenv("PI_CODING_AGENT_DIR", "~")
	if got, err := PiDir(); err != nil || got != home {
		t.Fatalf("home override: %q %v", got, err)
	}
}

func TestPiDirRejectsUnstableOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	for _, invalid := range []string{`\\?\` + home + `\config.\child`, `\\?\` + home + `\config \child`, home + `:stream`} {
		t.Setenv("PI_CODING_AGENT_DIR", invalid)
		if got, err := PiDir(); err == nil {
			t.Fatalf("unstable override accepted: %q", got)
		}
	}
}
