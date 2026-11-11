package explorer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func syncService(t testing.TB) *core.Service {
	t.Helper()
	home := t.TempDir()
	return &core.Service{Store: core.Store{Home: home}, FS: winfs.FS{}, Guard: core.DefaultGuard(home).WithCanonical(winfs.Canonical)}
}

func trackedFile(t testing.TB, dir, name string) core.Item {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := (winfs.FS{}).Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	return core.Item{Path: path, Identity: &identity, ExpiresAt: time.Date(2026, 11, 13, 0, 0, 0, 0, time.Local)}
}

func storeItems(t testing.TB, svc *core.Service, items ...core.Item) {
	t.Helper()
	for i := range items {
		items[i].ID, items[i].Rev = int64(i+1), 1
	}
	if err := svc.Store.Update(func(state *core.State) error {
		state.Items = items
		state.NextID = int64(len(items) + 1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
