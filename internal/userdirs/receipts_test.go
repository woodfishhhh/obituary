package userdirs

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceiptPathUsesNormalizedAbsolutePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OBIT_HOME", home)
	path := filepath.Join(t.TempDir(), "AGENTS.md.obituary-owners")
	got, err := ReceiptPath(path, "integration-receipts", ".obituary-owners")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "integration-receipts", fmt.Sprintf("%x.obituary-owners", sha256.Sum256([]byte(strings.ToLower(path)))))
	if got != want {
		t.Fatalf("got %s; want %s", got, want)
	}
	other, err := ReceiptPath(strings.ToUpper(path), "integration-receipts", ".obituary-owners")
	if err != nil || other != got {
		t.Fatal("ASCII alias has another key", other, err)
	}
	plain, _ := ReceiptPath(filepath.Join(home, "K"), "integration-receipts", "")
	unicode, _ := ReceiptPath(filepath.Join(home, "K"), "integration-receipts", "")
	if plain == unicode {
		t.Fatal("Unicode names conflated")
	}
}
