//go:build linux || darwin

package integrate

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPOSIXInstallationCommitPreservesExistingAndAbsentFiles(t *testing.T) {
	for _, mode := range []string{"create", "update", "remove"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "CLAUDE.md")
			before := "user notes\n"
			if mode != "create" {
				if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			change, err := readIntegrationChange(Scope{Global: true}, path)
			if err != nil {
				t.Fatal(err)
			}
			change.Action, change.data, change.delete = mode, []byte("installed\n"), mode == "remove"
			if err := Apply([]Change{change}); err != nil {
				t.Fatalf("commit failed: %v", err)
			}
			data, err := os.ReadFile(path)
			if mode == "remove" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("removed file still present: %q %v", data, err)
				}
			} else if err != nil || string(data) != "installed\n" {
				t.Fatalf("installation content: %q %v", data, err)
			}
			if mode != "create" && read(t, path+".bak") != before {
				t.Fatal("verified original was not backed up")
			}
			leftovers, err := filepath.Glob(filepath.Join(dir, ".obit-*.tmp"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("left commit temporaries: %v %v", leftovers, err)
			}
		})
	}
}

func TestPOSIXCommitPreservesEditorReplacement(t *testing.T) {
	for _, action := range []string{"create", "update", "remove"} {
		t.Run(action, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if action != "create" {
				writeIntegrationFile(t, path, "original")
			}
			change, err := readIntegrationChange(Scope{Global: true}, path)
			if err != nil {
				t.Fatal(err)
			}
			change.Action, change.data, change.delete = action, []byte("installer"), action == "remove"
			called := false
			beforeIntegrationCommit = func(string) error {
				called = true
				writeIntegrationFile(t, path+".editor", "editor replacement")
				return os.Rename(path+".editor", path)
			}
			t.Cleanup(func() { beforeIntegrationCommit = nil })
			if err := Apply([]Change{change}); err == nil || !called || read(t, path) != "editor replacement" {
				t.Fatalf("commit lost editor replacement: %v, called=%t", err, called)
			}
		})
	}
}

func TestPOSIXCommitRejectsChangedMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeIntegrationFile(t, path, "original")
	change, err := readIntegrationChange(Scope{Global: true}, path)
	if err != nil {
		t.Fatal(err)
	}
	change.Action, change.data = "update", []byte("installer")
	when := change.identity.ModTime().Add(time.Hour)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	if err := Apply([]Change{change}); err == nil || read(t, path) != "original" {
		t.Fatalf("commit accepted changed metadata: %v", err)
	}
}

func TestPOSIXCommitBackupsNeverOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeIntegrationFile(t, path, "original")
	writeIntegrationFile(t, path+".bak", "older original")
	for _, action := range []string{"update", "remove"} {
		change, err := readIntegrationChange(Scope{Global: true}, path)
		if err != nil {
			t.Fatal(err)
		}
		change.Action, change.data, change.delete = action, []byte("installed"), action == "remove"
		if err := Apply([]Change{change}); err != nil {
			t.Fatal(err)
		}
	}
	for suffix, want := range map[string]string{".bak": "older original", ".bak.1": "original", ".bak.2": "installed"} {
		if read(t, path+suffix) != want {
			t.Fatalf("backup overwritten: %s", suffix)
		}
	}
}

func TestPOSIXAgentHooksInstallAndUninstall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OBIT_USER_HOME", root)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	scope := Scope{Global: true}
	for _, uninstall := range []bool{false, true} {
		claude, err := PlanClaudeExec(scope, "/opt/obituary/obit", uninstall)
		if err != nil {
			t.Fatal(err)
		}
		codex, err := PlanCodex(scope, "'/opt/obituary/obit' hook codex", "", uninstall)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(append(claude, codex...)); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".codex", "hooks.json")} {
			data, err := os.ReadFile(path)
			if !uninstall && (err != nil || !strings.Contains(string(data), "/opt/obituary/obit")) {
				t.Fatalf("native hook missing: %s %v", path, err)
			}
			if uninstall && err == nil && strings.Contains(string(data), "/opt/obituary/obit") {
				t.Fatalf("hook remains after uninstall: %s", path)
			}
		}
	}
}

// Each mutation must compile and fail its intended observable assertion.
func TestPOSIXCommitMutations(t *testing.T) {
	if testing.Short() || os.Getenv("OBIT_TEST_POSIX_MUTATIONS") != "1" {
		t.Skip("set OBIT_TEST_POSIX_MUTATIONS=1 for compiled POSIX commit mutations")
	}
	for _, tc := range []struct{ name, from, to, test, assertion string }{
		{"final-check", "if _, err := p.verifiedPOSIXOriginal(c); err != nil {\n\t\treturn err\n\t}\n\tname :=", "if _, err := p.verifiedPOSIXOriginal(c); false && err != nil {\n\t\treturn err\n\t}\n\tname :=", "TestPOSIXCommitPreservesEditorReplacement/update", "commit lost editor replacement"},
		{"removal-identity", "if _, err := moved.verifiedPOSIXOriginal(c); err != nil {", "if _, err := moved.verifiedPOSIXOriginal(c); false && err != nil {", "TestPOSIXCommitPreservesEditorReplacement/remove", "commit lost editor replacement"},
		{"metadata", "if c.identity != nil && (info.Size()", "if false && c.identity != nil && (info.Size()", "TestPOSIXCommitRejectsChangedMetadata", "commit accepted changed metadata"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runPOSIXCommitMutation(t, tc.from, tc.to, tc.test, tc.assertion)
		})
	}
}

func runPOSIXCommitMutation(t *testing.T, from, to, test, assertion string) {
	t.Helper()
	source, err := filepath.Abs("apply_commit_unix.go")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil || strings.Count(string(data), from) != 1 {
		t.Fatalf("mutation anchor: %v", err)
	}
	dir := t.TempDir()
	mutant := filepath.Join(dir, "mutant.go")
	if err := os.WriteFile(mutant, []byte(strings.Replace(string(data), from, to, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{source: mutant}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "go", "test", "-overlay", path, "-run", "^"+test+"$", ".")
	out, err := cmd.CombinedOutput()
	if err == nil || strings.Contains(string(out), "[build failed]") || !strings.Contains(string(out), assertion) || !strings.Contains(string(out), "--- FAIL:") {
		t.Fatalf("expected compiled assertion failure: %v\n%s", err, out)
	}
	t.Logf("compiled mutation caught: %s", assertion)
}
