package gui

import (
	"path/filepath"
	"testing"
)

func TestOpenUsesRedirectedDirectoriesAndRejectsBrokenConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "home"))
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(root, "config"))
	a, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if a.deps.Service.Store.Home != filepath.Join(root, "home") || a.deps.ConfigDir != filepath.Join(root, "config") {
		t.Fatal("directory overrides ignored")
	}
	write(t, filepath.Join(a.deps.ConfigDir, "config.toml"), "bad = [")
	if _, err := Open(); err == nil {
		t.Fatal("invalid configuration accepted")
	}
}
