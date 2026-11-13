package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKiloCodeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("KILO_CONFIG_DIR", "")
	if got, err := AgentDir("kilocode"); err != nil || got != filepath.Join(home, ".config", "kilo") {
		t.Fatalf("default: %q %v", got, err)
	}
	xdg := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, err := AgentDir("kilocode"); err != nil || got != filepath.Join(xdg, "kilo") {
		t.Fatalf("XDG override: %q %v", got, err)
	}
	custom := filepath.Join(t.TempDir(), "custom")
	t.Setenv("KILO_CONFIG_DIR", custom)
	if got, err := AgentDir("kilocode"); err != nil || got != custom {
		t.Fatalf("Kilo override precedence: %q %v", got, err)
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatalf("resolver created directory: %v", err)
	}
}

func TestKiloCodeDirRejectsUnstablePrefixes(t *testing.T) {
	windowsDirectoryTest(t, "Win32 extended names, alternate streams, and trailing-dot normalization")

	for _, variable := range []string{"KILO_CONFIG_DIR", "XDG_CONFIG_HOME", "OBIT_USER_HOME"} {
		t.Run(variable, func(t *testing.T) {
			t.Setenv("KILO_CONFIG_DIR", "")
			t.Setenv("XDG_CONFIG_HOME", "")
			base := filepath.Join(t.TempDir(), "config")
			t.Setenv(variable, `\\?\`+base)
			got, err := KiloCodeDir()
			if err != nil || got == "" {
				t.Fatalf("extended spelling: %q %v", got, err)
			}
			for _, tail := range []string{".\\child", " \\child", ":stream", "\\NUL"} {
				t.Setenv(variable, `\\?\`+base+tail)
				if got, err := KiloCodeDir(); err == nil {
					t.Fatalf("unstable prefix accepted: %q", got)
				}
			}
		})
	}
}
