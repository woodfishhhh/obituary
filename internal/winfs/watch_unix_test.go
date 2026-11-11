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
