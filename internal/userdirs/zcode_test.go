package userdirs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/agentid"
)

func TestZCodeDirectory(t *testing.T) {
	home := t.TempDir()
	provider := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("ZCODE_DATA_BASE_DIR", provider)
	want := filepath.Join(home, ".zcode")
	for _, root := range nativeDirectorySpellings([]string{home, `\\?\` + home}) {
		t.Setenv("OBIT_USER_HOME", root)
		if got, err := AgentDir(agentid.ZCode); err != nil || got != want {
			t.Fatalf("runtime root must not follow provider override: %q %v", got, err)
		}
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("resolver must not create directories: %v", err)
	}
	for _, root := range nativeDirectorySpellings([]string{home + ":stream", `\\?\` + home + `.\child`}) {
		t.Setenv("OBIT_USER_HOME", root)
		if got, err := ZCodeDir(); err == nil {
			t.Fatalf("invalid directory accepted: %s", got)
		}
	}
}
