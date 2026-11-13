package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG", filepath.Join(home, "separate.json"))
	want := filepath.Join(home, ".config", "opencode")
	if got, err := AgentDir("opencode"); err != nil || got != want {
		t.Fatalf("default: %q %v", got, err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("resolver wrote directory: %v", err)
	}
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, err := OpenCodeDir(); err != nil || got != filepath.Join(xdg, "opencode") {
		t.Fatalf("XDG: %q %v", got, err)
	}
	custom := filepath.Join(home, "custom")
	t.Setenv("OPENCODE_CONFIG_DIR", custom)
	if got, err := AgentDir("opencode"); err != nil || got != custom {
		t.Fatalf("override precedence: %q %v", got, err)
	}
}

func TestOpenCodeConfigOverride(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG", "")
	if got, err := OpenCodeConfigOverride(); err != nil || got != "" {
		t.Fatalf("unset: %q %v", got, err)
	}
	path := filepath.Join(t.TempDir(), "config.jsonc")
	t.Run("windows-path-spelling", func(t *testing.T) {
		windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
		t.Setenv("OPENCODE_CONFIG", `\\?\`+path)
		if got, err := OpenCodeConfigOverride(); err != nil || got != path {
			t.Fatalf("override: %q %v", got, err)
		}
	})
	t.Run("windows-path-spelling", func(t *testing.T) {
		windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
		t.Setenv("OPENCODE_CONFIG", path+":stream")
		if _, err := OpenCodeConfigOverride(); err == nil {
			t.Fatal("accepted stream")
		}
	})
}

func TestOpenCodeDirValidatesOverrides(t *testing.T) {
	windowsDirectoryTest(t, "Win32 extended names, alternate streams, and trailing-dot normalization")

	for _, variable := range []string{"OPENCODE_CONFIG_DIR", "XDG_CONFIG_HOME"} {
		t.Run(variable, func(t *testing.T) {
			t.Setenv("OPENCODE_CONFIG_DIR", "")
			t.Setenv("XDG_CONFIG_HOME", "")
			path := filepath.Join(t.TempDir(), "config")
			t.Setenv(variable, `\\?\`+path)
			want := path
			if variable == "XDG_CONFIG_HOME" {
				want = filepath.Join(want, "opencode")
			}
			if got, err := OpenCodeDir(); err != nil || got != want {
				t.Fatalf("extended: %q %v", got, err)
			}
			for _, invalid := range []string{`\\?\` + path + `.\child`, `\\?\` + path + ` \child`, path + `:stream`} {
				t.Setenv(variable, invalid)
				if got, err := OpenCodeDir(); err == nil {
					t.Fatalf("unstable override accepted: %q", got)
				}
			}
		})
	}
}
