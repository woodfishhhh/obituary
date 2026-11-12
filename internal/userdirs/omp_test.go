package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOMPDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	for _, variable := range []string{"PI_CODING_AGENT_DIR", "PI_CONFIG_DIR", "OMP_PROFILE", "PI_PROFILE"} {
		t.Setenv(variable, "")
	}
	want := filepath.Join(home, ".omp", "agent")
	if got, err := AgentDir("omp"); err != nil || got != want {
		t.Fatalf("default: %q %v", got, err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("resolver wrote directory: %v", err)
	}
	t.Setenv("PI_CONFIG_DIR", ".omp-custom")
	if got, err := OMPDir(); err != nil || got != filepath.Join(home, ".omp-custom", "agent") {
		t.Fatalf("config name: %q %v", got, err)
	}
	custom := filepath.Join(home, "custom")
	for _, value := range nativeDirectorySpellings([]string{custom, `\\?\` + custom}) {
		t.Setenv("PI_CODING_AGENT_DIR", value)
		if got, err := OMPDir(); err != nil || got != custom {
			t.Fatalf("override precedence: %q %v", got, err)
		}
	}
	t.Setenv("PI_PROFILE", "work")
	if got, err := OMPDir(); err != nil || got != custom {
		t.Fatalf("explicit empty OMP_PROFILE did not select default: %q %v", got, err)
	}
	t.Setenv("OMP_PROFILE", "work")
	if got, err := OMPDir(); err == nil {
		t.Fatalf("named profile silently used default override: %q", got)
	}
	if err := os.Unsetenv("OMP_PROFILE"); err != nil {
		t.Fatal(err)
	}
	if got, err := OMPDir(); err == nil {
		t.Fatalf("legacy profile silently used default: %q", got)
	}
}

func TestOMPDirRejectsUnstableOverrides(t *testing.T) {
	windowsDirectoryTest(t, "Win32 extended names, alternate streams, and trailing-dot normalization")

	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_PROFILE", "")
	for _, invalid := range []string{`\\?\` + home + `\config.\child`, `\\?\` + home + `\config \child`, home + `:stream`} {
		t.Setenv("PI_CODING_AGENT_DIR", invalid)
		if got, err := OMPDir(); err == nil {
			t.Fatalf("unstable override accepted: %q", got)
		}
	}
	t.Setenv("PI_CODING_AGENT_DIR", "")
	for _, invalid := range []string{`..\outside`, `C:\absolute`, `config.`, `config `, `config \child`, `config:stream`} {
		t.Setenv("PI_CONFIG_DIR", invalid)
		if got, err := OMPDir(); err == nil {
			t.Fatalf("invalid config name accepted: %q", got)
		}
	}
}
