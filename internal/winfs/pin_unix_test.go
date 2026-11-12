//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestPOSIXImmediateWithoutBirthTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload")
	makePOSIXPayload(t, path, false, "keep")
	fs := FS{creationID: func(uint64) uint64 { return 0 }}
	id, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.MovePinned(path, id, filepath.Join(dir, "unscoped")); err == nil {
		t.Fatal("weak move without inspection pin accepted")
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: fs}
	rows, err := svc.Bury([]string{path}, false, "")
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionBuried || !strings.Contains(rows[0].Warning, "pinned") {
		t.Fatalf("immediate weak burial failed: %+v %v", rows, err)
	}
	grave := rows[0].Burial.Grave
	ref := fmt.Sprint(rows[0].Burial.ID)
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("weak burial did not move: %v", err)
	}
	rows, err = svc.PurgeRefs([]string{ref}, false)
	if err != nil || !core.GraveFailed(rows) {
		t.Fatalf("weak grave purged without durable identity: %+v %v", rows, err)
	}
	rows, err = svc.Resurrect([]string{ref})
	if err != nil || !core.GraveFailed(rows) {
		t.Fatalf("weak grave restored without durable identity: %+v %v", rows, err)
	}
	if data, err := os.ReadFile(grave); err != nil || string(data) != "keep" {
		t.Fatalf("weak grave changed: %q %v", data, err)
	}
}
