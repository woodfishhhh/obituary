package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClineDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("CLINE_DIR", "")
	t.Setenv("CLINE_DATA_DIR", "")
	want := filepath.Join(home, ".cline")
	if got, err := AgentDir("cline"); err != nil || got != want {
		t.Fatalf("default config: %q %v", got, err)
	}
	if got, err := ClineDataDir(); err != nil || got != filepath.Join(want, "data") {
		t.Fatalf("default data: %q %v", got, err)
	}
	custom := filepath.Join(t.TempDir(), "config")
	t.Setenv("CLINE_DIR", custom)
	if got, err := AgentDir("cline"); err != nil || got != custom {
		t.Fatalf("config override: %q %v", got, err)
	}
	if got, err := ClineDataDir(); err != nil || got != filepath.Join(custom, "data") {
		t.Fatalf("derived data: %q %v", got, err)
	}
	data := filepath.Join(t.TempDir(), "state")
	t.Setenv("CLINE_DATA_DIR", data)
	if got, err := ClineDataDir(); err != nil || got != data {
		t.Fatalf("data override: %q %v", got, err)
	}
	if got, err := ClineDir(); err != nil || got != custom {
		t.Fatalf("data override moved config: %q %v", got, err)
	}
	for _, dir := range []string{want, custom, data} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("resolver created directory: %s %v", dir, err)
		}
	}
}

func TestClineDirectoriesRejectUnstablePrefixes(t *testing.T) {
	for _, tc := range []struct {
		env string
		get func() (string, error)
	}{
		{"CLINE_DIR", ClineDir}, {"CLINE_DIR", ClineDataDir}, {"CLINE_DATA_DIR", ClineDataDir}, {"OBIT_USER_HOME", ClineDir},
	} {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("CLINE_DIR", "")
			t.Setenv("CLINE_DATA_DIR", "")
			base := filepath.Join(t.TempDir(), "config")
			t.Run("windows-path-spelling", func(t *testing.T) {
				windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
				t.Setenv(tc.env, `\\?\`+base)
				got, err := tc.get()
				if err != nil || got == "" {
					t.Fatalf("extended spelling: %q %v", got, err)
				}
			})
			for _, tail := range []string{".\\child", " \\child", ":stream", "\\NUL"} {
				t.Run("windows-path-spelling", func(t *testing.T) {
					windowsDirectoryTest(t, "Win32 extended names and alternate streams have no POSIX equivalent")
					t.Setenv(tc.env, `\\?\`+base+tail)
					if got, err := tc.get(); err == nil {
						t.Fatalf("unstable prefix accepted: %q", got)
					}
				})
			}
		})
	}
}
