package userdirs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHermesDir(t *testing.T) {
	home, local := filepath.Join(t.TempDir(), "home"), filepath.Join(t.TempDir(), "local")
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("HERMES_HOME", "")
	t.Setenv("HERMES_DATA_DIR_SUFFIX", "")
	want := filepath.Join(home, ".hermes")
	if runtime.GOOS == "windows" {
		want = filepath.Join(local, "hermes")
	}
	if got, err := AgentDir("hermes"); err != nil || got != want {
		t.Fatalf("default: %q %v, want %q", got, err, want)
	}
	t.Setenv("HERMES_DATA_DIR_SUFFIX", "-dev")
	if got, err := HermesDir(); err != nil || got != want+"-dev" {
		t.Fatalf("suffix: %q %v", got, err)
	}
	t.Setenv("HERMES_DATA_DIR_SUFFIX", "")
	t.Setenv("LOCALAPPDATA", "")
	if runtime.GOOS == "windows" {
		want = filepath.Join(home, "AppData", "Local", "hermes")
	}
	if got, err := HermesDir(); err != nil || got != want {
		t.Fatalf("home fallback: %q %v", got, err)
	}
	custom := filepath.Join(t.TempDir(), "profile")
	t.Setenv("HERMES_HOME", custom)
	if got, err := AgentDir("hermes"); err != nil || got != custom {
		t.Fatalf("override: %q %v", got, err)
	}
	t.Setenv("HERMES_DATA_DIR_SUFFIX", "-ignored")
	if got, err := HermesDir(); err != nil || got != custom {
		t.Fatalf("override precedence: %q %v", got, err)
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatalf("resolver created directory: %v", err)
	}
}

func TestHermesDirRejectsUnstablePrefixes(t *testing.T) {
	t.Setenv("HERMES_DATA_DIR_SUFFIX", "")
	for _, variable := range []string{"HERMES_HOME", "LOCALAPPDATA", "OBIT_USER_HOME"} {
		t.Run(variable, func(t *testing.T) {
			t.Setenv("HERMES_HOME", "")
			t.Setenv("LOCALAPPDATA", "")
			base := filepath.Join(t.TempDir(), "config")
			t.Setenv(variable, `\\?\`+base)
			got, err := HermesDir()
			if err != nil || got == "" {
				t.Fatalf("extended spelling: %q %v", got, err)
			}
			for _, tail := range []string{".\\child", " \\child", ":stream", "\\NUL"} {
				t.Setenv(variable, `\\?\`+base+tail)
				if got, err := HermesDir(); err == nil {
					t.Fatalf("unstable %s accepted: %q", variable, got)
				}
			}
		})
	}
	for _, placeholder := range []string{"~/.hermes", "%LOCALAPPDATA%/hermes", "$HOME/hermes"} {
		t.Setenv("HERMES_HOME", placeholder)
		if got, err := HermesDir(); err == nil {
			t.Fatalf("unexpanded path accepted: %q", got)
		}
	}
	for _, suffix := range []string{".", " ", `\..\other`, "/other", ":stream"} {
		t.Setenv("HERMES_HOME", "")
		t.Setenv("LOCALAPPDATA", t.TempDir())
		t.Setenv("HERMES_DATA_DIR_SUFFIX", suffix)
		if got, err := HermesDir(); err == nil {
			t.Fatalf("unstable suffix accepted: %q", got)
		}
	}
}
