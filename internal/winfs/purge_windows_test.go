package winfs

import (
	"os"
	"path/filepath"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func TestPurgeGraveChecksProtectedHandle(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "member")
	write(t, file)
	fs := FS{}
	root, err := fs.Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	member, err := fs.Identify(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.PurgeGrave(dir, root, corefs.ProtectedIDs{member.FileID: true}); err == nil {
		t.Fatal("protected member approved for removal")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
}
