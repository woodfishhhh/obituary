// Package testenv keeps tests and their child processes off real user data.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/runkey"
)

// Main runs a package's tests in disposable user and Git environments.
// The caller may redirect TEMP/TMPDIR to choose where the scratch is created.
func Main(m *testing.M) (code int) {
	restoreSync := core.SetFileSyncForTesting(func(*os.File) error { return nil })
	defer restoreSync()
	runkey.Current = NewRunRegistry()
	if os.Getenv("OBIT_TEST_FAKE_CLAUDE") == "1" && strings.EqualFold(filepath.Base(os.Args[0]), "claude.exe") {
		if err := runFakeClaude(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	desktop, err := realDesktopSnapshot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "test desktop snapshot:", err)
		return 1
	}
	defer func() {
		if !checkDesktop(desktop, realDesktopSnapshot, os.Stderr) {
			code = 1
		}
	}()
	cleanupRegistry, err := isolateRegistry()
	if err != nil {
		fmt.Fprintln(os.Stderr, "test registry isolation:", err)
		return 1
	}
	defer func() {
		if err := cleanupRegistry(); err != nil {
			fmt.Fprintln(os.Stderr, "test registry cleanup:", err)
			code = 1
		}
	}()
	// TEMP is an ordinary Windows path. Snapshot the real volume-root
	// graveyard read-only; tests must never create ledger keys outside scratch.
	graveyard := filepath.Join(pathutil.VolumeRoot(os.TempDir()), ".obituary", "graveyard")
	before := graveyardEntries(graveyard)
	defer func() {
		after := graveyardEntries(graveyard)
		for _, path := range newGraveyardEntries(before, after) {
			fmt.Fprintln(os.Stderr, "test leaked into volume graveyard:", path)
			code = 1
		}
	}()
	reapScratch(os.TempDir(), time.Now(), os.Stderr)
	scratch, err := os.MkdirTemp("", "obit-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "test isolation:", err)
		return 1
	}
	scratch = filepath.Clean(scratch)
	discardScratch := false
	defer func() {
		// Only this process's newly created scratch directory is removed.
		if err := os.RemoveAll(scratch); err != nil {
			fmt.Fprintln(os.Stderr, "test cleanup:", err)
			code = 1
			return
		}
		if code == 0 {
			ignored := ""
			if discardScratch {
				ignored = scratch
			}
			if err := compactTestLog(ignored); err != nil {
				fmt.Fprintln(os.Stderr, "test cache log:", err)
				code = 1
			}
		}
	}()
	if err := writeScratchOwner(scratch, scratchPackage()); err != nil {
		fmt.Fprintln(os.Stderr, "test scratch owner:", err)
		return 1
	}
	// Filesystem receipts use long names. Keep ordinary fixtures in that
	// spelling even when Windows supplies an 8.3 TEMP; alias tests create
	// their own short spelling explicitly.
	longScratch, err := pathutil.LongAbs(scratch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "test scratch path:", err)
		return 1
	}
	scratch = longScratch
	discardScratch = scratchOutsideModule(scratch)
	fakeClaudeScratch = scratch
	gitTemplateRoot = scratch
	if err := isolate(scratch); err != nil {
		fmt.Fprintln(os.Stderr, "test isolation:", err)
		return 1
	}
	code = m.Run()
	return code
}

func newGraveyardEntries(before, after map[string]bool) []string {
	if before == nil || after == nil {
		return nil // an unreadable snapshot cannot prove a new ledger key
	}
	var added []string
	for path := range after {
		if !before[path] {
			added = append(added, path)
		}
	}
	return added
}

func graveyardEntries(root string) map[string]bool {
	entries := make(map[string]bool)
	// A test Home has its own ledger key. Never recurse into another ledger:
	// the user's daemon may legitimately change or remove its contents.
	children, err := os.ReadDir(root)
	if err != nil {
		// Windows can report PATH_NOT_FOUND for an existing non-directory.
		// Only an absent root is an empty baseline; other failures are unknown.
		if _, statErr := os.Lstat(root); !os.IsNotExist(statErr) {
			return nil
		}
	}
	for _, child := range children {
		entries[filepath.Join(root, child.Name())] = true
	}
	return entries
}

func isolate(scratch string) error {
	if err := os.Setenv("OBIT_TEST_NO_UPDATE", "1"); err != nil {
		return err
	}
	if err := isolateDocker(scratch); err != nil {
		return err
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "GIT_") || upper == "OBIT_GUARD_RM" || upper == "OBIT_DISABLED" || upper == "OBIT_HOOK_AUDIT" || upper == "OBIT_SESSION_ID" || upper == "HERMES_DATA_DIR_SUFFIX" || upper == "OPENCODE_CONFIG" || upper == "OPENCLAW_CONFIG_PATH" || upper == "OPENCLAW_PROFILE" || upper == "PI_CONFIG_DIR" || upper == "CLAUDE_CODE_SESSION_ID" || upper == "CODEX_THREAD_ID" {
			if err := os.Unsetenv(name); err != nil {
				return err
			}
		}
	}
	for name, sub := range map[string]string{
		"OBIT_HOME": "data", "OBIT_CONFIG_DIR": "config", "OBIT_USER_HOME": "home",
		"LOCALAPPDATA":        "localappdata",
		"OBIT_START_MENU_DIR": "programs",
		"CLAUDE_CONFIG_DIR":   "claude", "CODEX_HOME": "codex",
		"QODER_CONFIG_DIR": "qoder", "QODERCN_CONFIG_DIR": "qoder-cn",
		"VIBE_HOME": "vibe", "KIMI_SHARE_DIR": "kimi",
		"DSH_HOME":     "deepseek",
		"COPILOT_HOME": "copilot", "GEMINI_CLI_HOME": "gemini-home", "FACTORY_HOME_OVERRIDE": "factory-home",
		"HERMES_HOME": "hermes", "CLINE_DIR": "cline", "CLINE_DATA_DIR": "cline-data", "KILO_CONFIG_DIR": "kilo",
		"OPENCODE_CONFIG_DIR": "opencode", "OPENCLAW_STATE_DIR": "openclaw", "OPENCLAW_HOME": "openclaw-home",
		"PI_CODING_AGENT_DIR": "pi", "XDG_CONFIG_HOME": "xdg-config",
		"TEMP": "tmp", "TMP": "tmp", "TMPDIR": "tmp", "GOTMPDIR": "tmp",

		"CODEBUDDY_CONFIG_DIR": "codebuddy",
		"MINIMAX_DATA_DIR":     "minimax", "MAVIS_DATA_DIR": "mavis",
		"MCODE_INSTALL_DIR":   "minimax-install",
		"ZCODE_DATA_BASE_DIR": "zcode-data",
	} {
		path := filepath.Join(scratch, sub)
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		if err := os.Setenv(name, path); err != nil {
			return err
		}
	}
	path := filepath.Join(scratch, "gitconfig")
	if err := os.WriteFile(path, []byte("[user]\n\tname = Obituary Tests\n\temail = tests@obituary.invalid\n[init]\n\tdefaultBranch = main\n[core]\n\tautocrlf = false\n"), 0o600); err != nil {
		return err
	}
	if err := os.Setenv("GIT_CONFIG_GLOBAL", path); err != nil {
		return err
	}
	// A scratch inside a checkout must not make non-repository fixtures inherit
	// that checkout; Git may still discover repositories created inside scratch.
	if err := os.Setenv("GIT_CEILING_DIRECTORIES", scratch); err != nil {
		return err
	}
	return os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}
