//go:build linux || darwin

package winfs

import (
	"encoding/hex"
	"errors"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"path/filepath"
	"testing"
)

func TestPOSIXMetadataRequiresPersistentIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index")
	makePOSIXPayload(t, path, false, "git metadata")
	want := posixID(t, path)
	stamp, err := ReadMetadata(path)
	if err != nil || hex.EncodeToString(stamp.FileID[:]) != want.FileID {
		t.Fatalf("metadata omitted persistent identity: %+v %v", stamp, err)
	}
	fs := FS{creationID: func(uint64) uint64 { return 0 }}
	if _, err := fs.readMetadata(path); !errors.Is(err, corefs.ErrUnsupported) {
		t.Fatalf("weak metadata stamp returned: %v", err)
	}
}
