package explorer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCleanupTargets(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing")
	if got, err := CleanupTargets(home); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("inspection created missing directory: %v", err)
	}
	home = t.TempDir()
	data := []byte(`{"b":{"path":"C:\\b"},"a":{"path":"C:\\a"},"duplicate":{"path":"C:\\a"}}`)
	index := filepath.Join(home, "explorer.json")
	if err := os.WriteFile(index, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := CleanupTargets(home)
	if err != nil || !reflect.DeepEqual(got, []string{`C:\a`, `C:\b`}) {
		t.Fatal(got, err)
	}
	if after, err := os.ReadFile(index); err != nil || string(after) != string(data) {
		t.Fatalf("inspection changed index: %s %v", after, err)
	}
	if err := os.WriteFile(index, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CleanupTargets(home); err == nil {
		t.Fatal("invalid index reported as empty")
	}
}
