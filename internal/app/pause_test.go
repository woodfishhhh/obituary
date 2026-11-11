//go:build windows

package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestPauseAppliesToOnceAndResumesExpiredItem(t *testing.T) {
	root := t.TempDir()
	home, path := filepath.Join(root, "data"), filepath.Join(root, "due.txt")
	if err := os.WriteFile(path, []byte("keep until resumed"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}, Now: func() time.Time { return time.Now().Add(-time.Hour) }}
	if _, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	svc.Now = time.Now
	if err := SetPaused(home, true); err != nil {
		t.Fatal(err)
	}
	if out, purged, err := Tick(svc); err != nil || len(out) != 0 || len(purged) != 0 {
		t.Fatalf("paused tick: %v %v %v", out, purged, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("paused item moved", err)
	}
	if err := SetPaused(home, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Tick(svc); err != nil {
		t.Fatal(err)
	}
	state, err := svc.Store.Read()
	if err != nil || len(state.Items) != 0 || len(state.Burials) != 1 {
		t.Fatalf("resume: %+v %v", state, err)
	}
}
