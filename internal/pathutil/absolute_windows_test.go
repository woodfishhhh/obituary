package pathutil_test

import (
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func TestAbsRejectsChangedVerbatimSpelling(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "payload.txt")
	for _, input := range []string{`\\?\` + base + ".", `\\?\` + base + " ", `\\?\` + filepath.Dir(base) + `\missing\..\payload.txt`, `\\?\` + filepath.Dir(base) + `\\payload.txt`, `\\?\` + filepath.Join(filepath.Dir(base), "nul"), `\\?\Volume{00000000-0000-0000-0000-000000000000}\x`, `\\?\GLOBALROOT\Device\HarddiskVolume1\x`} {
		if got, err := pathutil.Abs(input); err == nil {
			t.Fatalf("unsafe verbatim spelling normalized: %q => %q", input, got)
		}
	}
	if got, err := pathutil.Abs(`\\?\` + base); err != nil || got != base {
		t.Fatalf("safe absent verbatim name rejected: %q %v", got, err)
	}
}
