package contextmenu

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/i18n"
)

func TestNautilusPassesSelectionAsLiteralArguments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", home)
	program := filepath.Join(home, "Obituary name "+string(rune(39))+" & 100%")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf \"%s\\0\" \"\u0024@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	changes, err := PlanLanguage(UserRegistry(), program, false, i18n.English)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(UserRegistry(), changes); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "nautilus/scripts/Obituary/Add to Obituary · 7 days")
	out, err := exec.Command(path, "first file", "-second\nfile").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), strings.Join([]string{"mark", "--", "first file", "-second\nfile", "7d", ""}, "\x00"); got != want {
		t.Fatalf("argv: %q, want %q", got, want)
	}
}
