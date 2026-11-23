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

func TestMenuKeepsExplicitDataDirectory(t *testing.T) {
	home := t.TempDir()
	selected := filepath.Join(home, "isolated data")
	t.Setenv("OBIT_HOME", selected)
	t.Setenv("OBIT_USER_HOME", home)
	t.Setenv("XDG_DATA_HOME", home)
	exe := filepath.Join(home, "obit")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nprintf %s \"\u0024OBIT_HOME\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	reg := UserRegistry()
	changes, err := PlanLanguage(reg, exe, false, i18n.English)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(reg, changes); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(home, "nautilus/scripts/Obituary/Add to Obituary · 7 days"), "sample")
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	out, err := cmd.Output()
	if err != nil || string(out) != selected {
		t.Fatalf("desktop used %q, want %q: %v", out, selected, err)
	}
}
