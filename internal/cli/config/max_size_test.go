package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
)

func TestConfigMaxSizePreviewConfirmAndApply(t *testing.T) {
	clitest.IsolateRunner(t)
	repo, _ := clitest.CommandDir(t)
	svc, err := cmdutil.Open()
	if err != nil {
		t.Fatal(err)
	}
	f1 := filepath.Join(repo, "one.bin")
	f2 := filepath.Join(repo, "two.bin")
	if err := os.WriteFile(f1, bytes.Repeat([]byte("a"), 3000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, bytes.Repeat([]byte("b"), 3000), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Bury([]string{f1}, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Bury([]string{f2}, false, ""); err != nil {
		t.Fatal(err)
	}

	// Dry run previews eviction without changing config or deleting graves.
	out, stderr, err := clitest.Execute(t, NewCmd(), "--max-size", "4000", "--dry-run")
	if err != nil || !strings.Contains(out, `max_size = "4000"`) || !strings.Contains(stderr, "1") {
		t.Fatalf("dry-run: out=%q stderr=%q err=%v", out, stderr, err)
	}
	count, _, err := svc.GraveyardDiskUsage()
	if err != nil || count != 2 {
		t.Fatalf("dry-run deleted graves: count=%d err=%v", count, err)
	}

	// Non-terminal lowering without --yes refuses safely.
	oldTTY := isTerminal
	isTerminal = func() bool { return false }
	t.Cleanup(func() { isTerminal = oldTTY })
	if _, _, err := clitest.Execute(t, NewCmd(), "--max-size", "4000"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected non-TTY refusal, got %v", err)
	}

	// With --yes applies config and evicts the oldest grave.
	out, stderr, err = clitest.Execute(t, NewCmd(), "--max-size", "4000", "--yes")
	if err != nil || !strings.Contains(out, `max_size = "4000"`) || !strings.Contains(stderr, "1") {
		t.Fatalf("apply with --yes: out=%q stderr=%q err=%v", out, stderr, err)
	}
	count, used, err := svc.GraveyardDiskUsage()
	if err != nil || count != 1 || used != 3000 {
		t.Fatalf("after apply: count=%d used=%d err=%v", count, used, err)
	}

	// Plain `obit config` shows max_size under [graveyard].
	showOut, _, err := clitest.Execute(t, NewCmd())
	if err != nil || !strings.Contains(showOut, `max_size = "4000"`) {
		t.Fatalf("config show omitted max_size: %s %v", showOut, err)
	}

	// Invalid value fails cleanly.
	if _, _, err := clitest.Execute(t, NewCmd(), "--max-size", "-5GB"); err == nil {
		t.Fatal("expected negative --max-size to fail")
	}

	// --language and --max-size are mutually exclusive (prevents partial writes on --dry-run).
	if _, _, err := clitest.Execute(t, NewCmd(), "--language", "en", "--max-size", "4000", "--dry-run"); err == nil {
		t.Fatal("expected --language + --max-size to be rejected")
	}
}

func TestConfigMaxSizeDryRunDoesNotCreateLockFiles(t *testing.T) {
	clitest.IsolateRunner(t)
	emptyHome := filepath.Join(t.TempDir(), "fresh-home")
	cfgDir := t.TempDir()
	t.Setenv("OBIT_HOME", emptyHome)
	t.Setenv("OBIT_CONFIG_DIR", cfgDir)
	out, _, err := clitest.Execute(t, NewCmd(), "--max-size", "4000", "--dry-run")
	if err != nil || !strings.Contains(out, `max_size = "4000"`) {
		t.Fatalf("dry-run on empty home: out=%q err=%v", out, err)
	}
	if _, err := os.Stat(emptyHome); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not create data dir or .lock files: stat=%v", err)
	}
}
