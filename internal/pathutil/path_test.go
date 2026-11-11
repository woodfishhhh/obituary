//go:build windows

package pathutil_test

import (
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func TestVolumeRootIncludesExtendedUNCShare(t *testing.T) {
	t.Parallel()
	for _, path := range []string{`\\?\UNC\server\share\x`, `\\?\unc\server\share\missing\x`} {
		want := path[:len(`\\?\UNC\server\share\`)]
		if got := pathutil.VolumeRoot(path); got != want {
			t.Fatalf("extended UNC volume root omitted share: %q", got)
		}
	}
}

func TestParentStopsAtVolumeRoot(t *testing.T) {
	t.Parallel()
	for _, root := range []string{`C:\`, `\\?\C:\`, `\\server\share`, `\\?\UNC\server\share`, `\\?\unc\server\share`} {
		if got := pathutil.Parent(root); got != root {
			t.Fatalf("parent crossed volume root: %q => %q", root, got)
		}
		if got := pathutil.Parent(root + `\child`); strings.TrimRight(got, `\`) != strings.TrimRight(root, `\`) {
			t.Fatalf("child parent = %q, want %q", got, root)
		}
	}
}
