package watch_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func approvePolicies(t *testing.T, svc *core.Service) {
	t.Helper()
	st, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range st.Proposals {
		rows, err := (&auto.Engine{Svc: svc}).Approve([]string{fmt.Sprint(p.ID)})
		if err != nil || len(rows) != 1 || rows[0].Error != "" {
			t.Fatalf("approve: %+v %v", rows, err)
		}
	}
}

func fixture(t *testing.T) (*core.Service, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "downloads")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, FS: winfs.FS{}, Owner: auto.Owner}
	return svc, dir
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
}
