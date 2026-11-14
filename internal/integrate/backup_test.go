package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestProjectBackupCreatesMissingDataDirectory(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing-data")
	t.Setenv("OBIT_HOME", home)
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	writeIntegrationFile(t, path, "user original\n")
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("planning created the data directory: %v", err)
	}
	applyIntegration(t, []Change{setIntegrationData(ch, []byte("updated\n"))}, nil)
	if read(t, testBackupPath(t, path)) != "user original\n" || read(t, path) != "updated\n" {
		t.Fatal("first project update lost original or new content")
	}
}

func TestProjectBackupsPreserveOriginalOutsideProject(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	writeIntegrationFile(t, path, "user original\n")
	for _, content := range []string{"first update\n", "second update\n"} {
		ch, err := readIntegrationChange(s, path)
		if err != nil {
			t.Fatal(err)
		}
		applyIntegration(t, []Change{setIntegrationData(ch, []byte(content))}, nil)
	}
	backup := testBackupPath(t, path)
	if read(t, backup) != "user original\n" || read(t, backup+".1") != "first update\n" {
		t.Fatal("numbered backups lost originals")
	}
	entries, err := os.ReadDir(s.Project)
	if err != nil || len(entries) != 1 || entries[0].Name() != "AGENTS.md" {
		t.Fatal("backup polluted project", entries, err)
	}
}

// obit init's copies of project files leave the data directory after the
// lifespan of Obituary's own backups; nothing else there is touched.
func TestPruneBackupsAgesOutProjectCopies(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "integration-backups")
	now := time.Now()
	hash := strings.Repeat("ab", 32)
	ages := map[string]time.Duration{
		hash + ".bak":   ledger.BackupLifespan + time.Hour,
		hash + ".bak.2": ledger.BackupLifespan + time.Hour,
		hash + ".bak.3": time.Hour,
		"notes.txt":     ledger.BackupLifespan + time.Hour,
	}
	for name, age := range ages {
		writeIntegrationFile(t, filepath.Join(dir, name), "x")
		if err := os.Chtimes(filepath.Join(dir, name), now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	if err := PruneBackups(home, now); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{hash + ".bak": false, hash + ".bak.2": false, hash + ".bak.3": true, "notes.txt": true} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err == nil) != want {
			t.Errorf("%s kept=%v, want %v", name, err == nil, want)
		}
	}
	if err := PruneBackups(t.TempDir(), now); err != nil {
		t.Fatalf("no backups yet: %v", err)
	}
}
