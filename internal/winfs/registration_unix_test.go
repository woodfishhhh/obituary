//go:build linux || darwin

package winfs

import (
	"errors"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func TestFileAncestryUnavailableOnPOSIX(t *testing.T) {
	proof, err := (FS{}).OpenFileAncestry("/unused", corefs.Identity{})
	if proof != nil || !errors.Is(err, corefs.ErrUnsafe) {
		t.Fatalf("unsupported ancestry proof: %v %v", proof, err)
	}
}
