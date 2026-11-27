//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"golang.org/x/sys/unix"
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
	svc := &lifecycle.Service{Store: ledger.Store{Home: filepath.Join(dir, "data")}, FS: fs}
	rows, err := svc.Bury([]string{path}, false, "")
	if err != nil || lifecycle.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != ledger.ActionBuried || !strings.Contains(rows[0].Warning, "pinned") {
		t.Fatalf("immediate weak burial failed: %+v %v", rows, err)
	}
	grave := rows[0].Burial.Grave
	ref := fmt.Sprint(rows[0].Burial.ID)
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("weak burial did not move: %v", err)
	}
	rows, err = svc.PurgeRefs([]string{ref}, false)
	if err != nil || !lifecycle.GraveFailed(rows) {
		t.Fatalf("weak grave purged without durable identity: %+v %v", rows, err)
	}
	rows, err = svc.Resurrect([]string{ref})
	if err != nil || !lifecycle.GraveFailed(rows) {
		t.Fatalf("weak grave restored without durable identity: %+v %v", rows, err)
	}
	if data, err := os.ReadFile(grave); err != nil || string(data) != "keep" {
		t.Fatalf("weak grave changed: %q %v", data, err)
	}
}

func TestWeakImmediateDirectory(t *testing.T) {
	for _, kind := range []string{"empty", "nonempty", "active", "active-empty", "active-weak-child"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "tree")
			if err := os.Mkdir(p, 0700); err != nil {
				t.Fatal(err)
			}
			if kind != "empty" && kind != "active-empty" {
				makePOSIXPayload(t, filepath.Join(p, "child"), false, "keep")
			}
			svc := &lifecycle.Service{Store: ledger.Store{Home: filepath.Join(dir, "data")}, FS: FS{}}
			if strings.HasPrefix(kind, "active") {
				other := filepath.Join(dir, "other")
				makePOSIXPayload(t, other, false, "keep")
				rs, e := svc.Add([]string{other}, lifecycle.AddOptions{ExpiresAt: time.Now().Add(24 * time.Hour)})
				if e != nil || lifecycle.Failed(rs) {
					t.Fatal(rs, e)
				}
			}
			backend := FS{creationID: func(uint64) uint64 { return 0 }}
			if kind == "active-weak-child" {
				child, err := (FS{}).statAt(unix.AT_FDCWD, filepath.Join(p, "child"))
				if err != nil {
					t.Fatal(err)
				}
				backend.creationID = func(n uint64) uint64 {
					if n == child.Incarnation {
						return 0
					}
					return n
				}
			}
			svc.FS = backend
			rows, err := svc.Bury([]string{p}, true, "")
			if err != nil || len(rows) != 1 {
				t.Fatal(rows, err)
			}
			if strings.HasPrefix(kind, "active") {
				if _, err := os.Stat(p); err != nil || rows[0].Action != ledger.ActionRetry || !strings.Contains(rows[0].Error, "active") || strings.Contains(rows[0].Error, "obit bury") {
					t.Fatalf("weak protected tree: %+v", rows)
				}
			} else if rows[0].Action != ledger.ActionBuried {
				t.Fatalf("weak immediate tree: %+v", rows)
			}
		})
	}
}
