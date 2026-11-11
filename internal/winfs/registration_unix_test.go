//go:build linux || darwin

package winfs

import (
	"errors"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestFileAncestryUnavailableOnPOSIX(t *testing.T) {
	proof, err := (FS{}).OpenFileAncestry("/unused", core.Identity{})
	if proof != nil || !errors.Is(err, core.ErrUnsafe) {
		t.Fatalf("unsupported ancestry proof: %v %v", proof, err)
	}
}
