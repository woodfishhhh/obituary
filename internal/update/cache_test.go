package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCacheRejectsCorruption(t *testing.T) {
	for _, text := range []string{`{`, `{"latest":{"version":"1.0.0-rc.1"}}`, `{"latest":{"version":"1.0.0","url":"javascript:alert(1)"}}`} {
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, "update.json"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(home); err == nil {
			t.Fatalf("invalid cache accepted: %s", text)
		}
	}
}
