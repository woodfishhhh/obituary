package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTraeDirectories(t *testing.T) {
	for _, tc := range []struct{ name, leaf string }{{"trae", ".trae"}, {"trae-cn", ".trae-cn"}} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("OBIT_USER_HOME", home)
			want := filepath.Join(home, tc.leaf)
			if got, err := AgentDir(tc.name); err != nil || got != want {
				t.Fatalf("default: %q %v", got, err)
			}
			if _, err := os.Stat(want); !os.IsNotExist(err) {
				t.Fatalf("resolver created directory: %v", err)
			}
			t.Setenv("OBIT_USER_HOME", `\\?\`+home)
			if got, err := AgentDir(tc.name); err != nil || got != want {
				t.Fatalf("extended home: %q %v", got, err)
			}
			t.Setenv("OBIT_USER_HOME", `\\?\`+home+`.\child`)
			if got, err := AgentDir(tc.name); err == nil {
				t.Fatalf("unstable home accepted: %q", got)
			}
		})
	}
}
