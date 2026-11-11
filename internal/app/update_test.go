package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackgroundUpdatesDisabledDoNotCreateMetadata(t *testing.T) {
	t.Setenv("OBIT_TEST_NO_UPDATE", "")
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", root)
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("[update]\ncheck=false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "absent")
	CheckUpdatesOnce(home)
	checkUpdates(context.Background(), home)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stop := startUpdateChecks(ctx, home)
	stop()
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("disabled/cancelled check wrote metadata: %v", err)
	}
}

func TestBackgroundUpdateUsesFreshCache(t *testing.T) {
	t.Setenv("OBIT_TEST_NO_UPDATE", "")
	root := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", root)
	data := []byte(`{"checkedAt":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`)
	path := filepath.Join(root, "update.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	CheckUpdatesOnce(root)
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data) {
		t.Fatalf("fresh cache changed: %s %v", got, err)
	}
}
