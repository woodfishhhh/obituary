package userdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQoderDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("QODER_CONFIG_DIR", "")
	t.Setenv("QODERCN_CONFIG_DIR", "")
	for _, tc := range []struct{ name, env, leaf string }{
		{"qoder", "QODER_CONFIG_DIR", ".qoder"}, {"qoder-cn", "QODERCN_CONFIG_DIR", ".qoder-cn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := filepath.Join(home, tc.leaf)
			if got, err := AgentDir(tc.name); err != nil || got != want {
				t.Fatalf("default: %q %v", got, err)
			}
			custom := filepath.Join(t.TempDir(), "custom")
			for _, path := range []string{custom, `\\?\` + custom} {
				t.Setenv(tc.env, path)
				if got, err := AgentDir(tc.name); err != nil || got != custom {
					t.Fatalf("override: %q %v", got, err)
				}
			}
			for _, path := range []string{want, custom} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("resolver wrote", path, err)
				}
			}
			for _, path := range []string{custom + ":stream", `\\?\` + custom + `.\child`} {
				t.Setenv(tc.env, path)
				if got, err := AgentDir(tc.name); err == nil {
					t.Fatal("unsafe override accepted", got)
				}
			}
		})
	}
}
