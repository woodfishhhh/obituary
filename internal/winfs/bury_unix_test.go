//go:build linux || darwin

package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPOSIXMovesFailWithoutTouchingObjects(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "source"), filepath.Join(dir, "target")
	for _, path := range []string{source, target} {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fs := FS{}
	id, err := fs.Identify(source)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := fs.OpenRestoreParent(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	for _, err := range []error{fs.MovePinned(source, id, target), fs.Recycle(source, id)} {
		if !errors.Is(err, core.ErrUnsupported) {
			t.Fatal(err)
		}
	}
	if _, err := fs.MovePinnedInto(source, id, parent, "target"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := fs.MkdirRestoreParent(parent, "new"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "new")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unsupported restore created a parent", err)
	}
	for _, path := range []string{source, target} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != path {
			t.Fatalf("object changed: %s %q %v", path, data, err)
		}
	}
}

func TestPOSIXUnsupportedBurialRetainsTrackedEntry(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "payload")
	if err := os.WriteFile(source, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}, Now: func() time.Time { return now }}
	rows, err := svc.Add([]string{source}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("register: %+v %v", rows, err)
	}
	now = now.Add(2 * time.Hour)
	out, err := svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("sweep: %+v %v", out, err)
	}
	state, err := svc.Store.ReadSnapshot()
	if err != nil || len(state.Items) != 1 || len(state.Burials) != 0 || !strings.Contains(state.Items[0].LastError, "unsupported") && !strings.Contains(state.Items[0].LastError, "not supported") {
		t.Fatalf("entry discarded: %+v %v", state, err)
	}
	data, err := os.ReadFile(source)
	if err != nil || string(data) != "keep" {
		t.Fatalf("payload changed: %q %v", data, err)
	}
	buried, err := svc.Bury([]string{source}, false, "")
	if err == nil && !core.GraveFailed(buried) {
		t.Fatalf("unsupported burial reported success: %+v", buried)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
}

func TestPOSIXMakeGraveIsExclusive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "graveyard", "one")
	fs := FS{}
	if err := fs.MakeGrave(dir); err != nil {
		t.Fatal(err)
	}
	if err := fs.MakeGrave(dir); !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
}
