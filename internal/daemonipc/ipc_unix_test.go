//go:build !windows

package daemonipc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestLongHomeConnectsToSameResident(t *testing.T) {
	home := filepath.Join(t.TempDir(), strings.Repeat("a", 150))
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	stop, err := Serve(home, func([]core.Observation) error { return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Submit(context.Background(), home, []core.Observation{{Token: "test"}}); err != nil {
		t.Error(err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

func TestListenPreservesNonSocket(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "resident.sock")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	listener, cleanup, _ := listen(home)
	if listener != nil {
		_ = listener.Close()
		_ = cleanup()
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("existing file not preserved: %q, %v", data, err)
	}
}
