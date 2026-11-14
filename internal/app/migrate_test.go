package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// Upgrading turns automatic registration off once and spares the
// dependencies it had registered; a later choice to turn it on stands.
func TestMigrationTurnsAutomaticRegistrationOffOnce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", filepath.Join(dir, "config"))
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(path, "[proposals]\nmode = \"tiered\"\n")
	root := filepath.Join(dir, "repo")
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: winfs.FS{}}
	add := func(rel, source string) {
		t.Helper()
		p := filepath.Join(root, rel)
		write(filepath.Join(p, "x"), "x")
		rows, err := svc.Add([]string{p}, core.AddOptions{ExpiresAt: time.Now().Add(24 * time.Hour), Source: source, Project: root})
		if err != nil || core.Failed(rows) {
			t.Fatalf("add %s: %v %+v", rel, err, rows)
		}
	}
	add("node_modules", "rule")       // automatic dependency: spared
	add("test-results", "rule")       // automatic, not a dependency: kept
	add(filepath.Join("dist"), "cli") // the user's own mark: kept
	if err := migrateAutoOff(svc); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil || cfg.Proposals.Mode != config.ProposalsOff {
		t.Fatalf("mode after upgrade: %+v %v", cfg.Proposals, err)
	}
	st, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	kept := map[string]string{}
	for _, it := range st.Items {
		kept[filepath.Base(it.Path)] = it.Source
	}
	if len(kept) != 3 || kept["test-results"] != "rule" || kept["dist"] != "cli" || kept["config.toml.bak"] != "init" {
		t.Fatalf("items after upgrade: %v", kept)
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules", "x")); err != nil {
		t.Fatalf("spared dependency touched: %v", err)
	}
	write(path, "[proposals]\nmode = \"tiered\"\n")
	if err := migrateAutoOff(svc); err != nil {
		t.Fatal(err)
	}
	if cfg, err := config.Load(); err != nil || cfg.Proposals.Mode != config.ProposalsTiered {
		t.Fatalf("the user's later choice was undone: %+v %v", cfg.Proposals, err)
	}
}

// upgraded marks home as past the 1.3 change, so a tick keeps the test's mode.
func upgraded(t *testing.T, home string) {
	t.Helper()
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, autoOffMarker), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}
