package pathutil_test

import (
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func TestAbsReservedFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, name := range []string{"NUL", "nul.txt", "CON", "prn", "AUX", "com1", "lpt9.log", "COM¹.txt"} {
		want := `\\?\` + filepath.Join(dir, name)
		for _, input := range []string{name, "./" + name, filepath.Join(dir, name), want} {
			if got, err := pathutil.Abs(input); err != nil || got != want {
				t.Errorf("Abs(%q) = %q, %v; want %q", input, got, err, want)
			}
		}
	}
	for _, input := range []string{`\\.\NUL`, `//./NUL`, `\\.\NUL\child`, `NUL:stream`, `NUL.`, `NUL `, `\\?\` + dir + `\..\NUL`} {
		if got, err := pathutil.Abs(input); err == nil {
			t.Errorf("ambiguous/device operand accepted: %q => %q", input, got)
		}
	}
}

func TestAbsRejectsChangedVerbatimSpelling(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "payload.txt")
	for _, input := range []string{`\\?\` + base + ".", `\\?\` + base + " ", `\\?\` + filepath.Dir(base) + `\missing\..\payload.txt`, `\\?\` + filepath.Dir(base) + `\\payload.txt`, `\\?\Volume{00000000-0000-0000-0000-000000000000}\x`, `\\?\GLOBALROOT\Device\HarddiskVolume1\x`} {
		if got, err := pathutil.Abs(input); err == nil {
			t.Fatalf("unsafe verbatim spelling normalized: %q => %q", input, got)
		}
	}
	if got, err := pathutil.Abs(`\\?\` + base); err != nil || got != base {
		t.Fatalf("safe absent verbatim name rejected: %q %v", got, err)
	}
}
