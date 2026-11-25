//go:build linux || darwin

package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func TestPOSIXWatchObservationDoesNotPromiseWriterExclusion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "download")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := FS{}
	id, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, release, err := fs.ObserveWatch(path, false)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if snapshot.Identity != id || snapshot.Size != 7 || snapshot.Directory || snapshot.Modified.IsZero() {
		t.Fatal(snapshot)
	}
	if _, release, err := fs.ObserveWatch(path, true); !errors.Is(err, corefs.ErrUnsupported) || release != nil {
		t.Fatalf("claimed writer exclusion: %v", err)
	}
	link := path + "-link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, release, err := fs.ObserveWatch(link, false); !errors.Is(err, corefs.ErrUnsafe) || release != nil {
		t.Fatalf("followed symbolic link: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "payload" {
		t.Fatal("observation changed file", err)
	}
}

func TestWatchInventoryDecision(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		owner, current, mode uint32
		private              bool
	}{
		{"owner-only-write", 1000, 1000, 0644, true}, {"group-writable", 1000, 1000, 0664, false}, {"world-writable", 1000, 1000, 0602, false}, {"other-owner", 1001, 1000, 0600, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := privateWatchFile(tc.owner, tc.current, tc.mode); got != tc.private {
				t.Fatalf("private: %v, want %v", got, tc.private)
			}
		})
	}
	var first error
	if recordWatchError(&first, os.ErrPermission) || first == nil {
		t.Fatal("stopped before scanning other descriptors")
	}
	if watchInventoryResult(false, first) == nil || watchInventoryResult(true, first) != nil {
		t.Fatal("only approval can override an unresolved inventory")
	}
	if !recordWatchError(&first, errWatchFileOpen) || !errors.Is(watchInventoryResult(true, first), errWatchFileOpen) {
		t.Fatal("approval hid known open file after an unreadable process")
	}
}
