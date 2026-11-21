package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRetireUpdateFilesGivesTheFolderADayAndNothingElse(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	folder := filepath.Join(bin, ".obituary-update-abc", "old")
	for path, data := range map[string]string{
		filepath.Join(folder, "obit"):                       "previous",
		filepath.Join(bin, "jq"):                            "another tool",
		filepath.Join(bin, "obit"):                          "current",
		filepath.Join(filepath.Dir(folder), "new", ".keep"): "",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	update := filepath.Dir(folder)
	if err := retireUpdateFiles([]string{update}); err != nil {
		t.Fatal(err)
	}
	svc, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	state, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Items) != 1 {
		t.Fatalf("registered %d items, want only the update folder", len(state.Items))
	}
	item := state.Items[0]
	if filepath.Clean(item.Path) != update || item.Source != "update" || time.Until(item.ExpiresAt) < 23*time.Hour || time.Until(item.ExpiresAt) > 25*time.Hour {
		t.Fatalf("item %+v", item)
	}
	for _, kept := range []string{filepath.Join(bin, "jq"), filepath.Join(bin, "obit")} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s: %v", kept, err)
		}
	}
	// A path that does not exist is an error, not a silent success.
	if err := retireUpdateFiles([]string{filepath.Join(bin, "missing")}); err == nil {
		t.Error("a missing folder was accepted")
	}
}
