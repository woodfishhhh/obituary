//go:build windows

package e2e_test

import (
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestExplorerMenuCommandFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	repo, store := commandRepo(t)
	// Notifications follow [ui] language; pin it so the English CI runner
	// sees the same title as a Chinese desktop.
	writeFixture(t, filepath.Join(os.Getenv("OBIT_CONFIG_DIR"), "config.toml"), "[ui]\nlanguage = \"zh-CN\"\n")
	paths := []string{filepath.Join(repo, "报告 & %PATH% ! ' $(echo) 😀.txt"), filepath.Join(repo, "two files", "second.txt")}
	for _, path := range paths {
		writeFixture(t, path, "keep me")
	}
	notices := 0
	notify := func(title, message string) error {
		notices++
		if title != "讣告：操作完成" || message == "" {
			t.Fatal(title, message)
		}
		return nil
	}
	for _, lifespan := range []struct {
		arg      string
		duration time.Duration
	}{{"1h", time.Hour}, {"1d", 24 * time.Hour}, {"7d", 7 * 24 * time.Hour}, {"30d", 30 * 24 * time.Hour}} {
		before := time.Now()
		args := append([]string{binaryPath(t, "obit"), lifespan.arg}, paths...)
		if err := contextmenu.Run(args, contextmenu.Execute, notify); err != nil {
			t.Fatal(err)
		}
		st, err := store.ReadSnapshot()
		if err != nil || len(st.Items) != 2 {
			t.Fatal(st, err)
		}
		for _, item := range st.Items {
			if item.ExpiresAt.Before(before.Add(lifespan.duration)) || item.ExpiresAt.After(time.Now().Add(lifespan.duration)) {
				t.Fatal(item)
			}
			if data, err := os.ReadFile(item.Path); err != nil || string(data) != "keep me" {
				t.Fatal(item.Path, err)
			}
		}
	}
	if out, stderr, err := execute(t, "bury", paths[1]); err != nil {
		t.Fatal(out, stderr, err)
	}
	if err := contextmenu.Run([]string{binaryPath(t, "obit"), "resurrect", paths[0]}, contextmenu.Execute, notify); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(paths[1]); err != nil || string(data) != "keep me" {
		t.Fatal(string(data), err)
	}
	if err := contextmenu.Run([]string{binaryPath(t, "obit"), "policy", repo}, contextmenu.Execute, notify); err != nil {
		t.Fatal(err)
	}
	if notices != 6 {
		t.Fatal("notification count", notices)
	}
	st, err := store.ReadSnapshot()
	if err != nil || len(st.Burials) != 0 {
		t.Fatal(st.Burials, err)
	}
	if len(st.Watches) != 1 || st.Watches[0].Consent != "pending" || len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalWatch || len(st.Items) != 1 {
		t.Fatalf("Explorer policy bypassed approval: %+v", st)
	}
}

func TestExplorerCompanionIsGUIAndInvalidActionDoesNotStartDaemon(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	path := binaryPath(t, "obitd")
	file, err := pe.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var subsystem uint16
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		subsystem = header.Subsystem
	case *pe.OptionalHeader32:
		subsystem = header.Subsystem
	}
	if subsystem != pe.IMAGE_SUBSYSTEM_WINDOWS_GUI {
		t.Fatal("companion has console subsystem", subsystem)
	}
	home := filepath.Join(t.TempDir(), "data")
	t.Setenv("OBIT_HOME", home)
	cmd := exec.Command(path, "--context-menu", binaryPath(t, "obit"), "invalid", "file")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "unknown context menu action") {
		t.Fatal(string(out), err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("menu entered daemon path", err)
	}
}
