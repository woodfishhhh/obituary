package integrate

import (
	"os"
	"path/filepath"
	"testing"
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
