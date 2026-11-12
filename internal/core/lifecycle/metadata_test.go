package lifecycle

import (
	"path/filepath"
	"testing"
)

func TestMetadataAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	for _, text := range []string{`{"version":"1.0.0"}`, `{"version":"1.0.1"}`} {
		if err := WriteMetadata(path, []byte(text)); err != nil {
			t.Fatal(err)
		}
		got, err := ReadMetadata(path)
		if err != nil || string(got) != text {
			t.Fatalf("metadata: %s %v", got, err)
		}
	}
}
