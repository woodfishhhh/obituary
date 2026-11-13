package integrate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestInstallRejectsProjectRootSwap(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, agent := range []string{"copilot", "cursor", "gemini", "droid", "trae", "codebuddy", "qoder", "zcode", "workbuddy", "antigravity", "opencode", "pi", "roo"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			root := filepath.Join(parent, "project")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			commands := Commands{Native: testenv.NativeFixture(`"C:/tools/obit.exe" hook `) + agent, Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}
			plan, err := PlanAgent(agent, Scope{Project: root}, commands, false)
			if err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			if err := os.Rename(root, filepath.Join(parent, "original")); err != nil {
				t.Fatal(err)
			}
			makeIntegrationJunction(t, root, outside)
			t.Cleanup(func() { _ = os.Remove(root) })
			err = Apply(plan)
			entries, readErr := os.ReadDir(outside)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) > 0 || err == nil {
				t.Fatalf("project-root junction followed: Apply=%v outside entries=%v", err, entries)
			}
		})
	}
}

func TestInstallRechecksParentsAfterPreflight(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, hardlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "equal-bytes", true: "same-file"}[hardlink], func(t *testing.T) {
			t.Parallel()
			root, outside := t.TempDir(), t.TempDir()
			ruleRel := filepath.Join(".roo", "rules", "obituary.md")
			original := "user-specific instructions\n" + blockBegin + "\nold Obituary instructions\n" + blockEnd + "\n"
			writeIntegrationFile(t, filepath.Join(root, ruleRel), original)
			writeIntegrationFile(t, filepath.Join(outside, "rules", "obituary.md"), original)
			if hardlink {
				path := filepath.Join(outside, "rules", "obituary.md")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(filepath.Join(root, ruleRel), path); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := PlanRoo(Scope{Project: root}, Commands{}, true)
			if err != nil {
				t.Fatal(err)
			}
			gate := filepath.Join(root, "gate.txt")
			plan = append([]Change{{Path: gate, Action: "create", data: []byte("gate"), project: root, phase: -100}}, plan...)
			swapped := false
			afterIntegrationWrite := func(path string) error {
				if path != gate {
					return nil
				}
				dir := filepath.Join(root, ".roo")
				if err := os.Rename(dir, filepath.Join(root, ".roo-original")); err != nil {
					return err
				}
				makeIntegrationJunction(t, dir, outside)
				swapped = true
				return nil
			}
			t.Cleanup(func() { _ = os.Remove(filepath.Join(root, ".roo")) })
			err = apply(plan, afterIntegrationWrite)
			if !swapped {
				t.Fatalf("directory swap did not execute: %v", err)
			}
			got := read(t, filepath.Join(outside, "rules", "obituary.md"))
			if got != original || err == nil {
				t.Fatalf("outside rules overwritten after preflight: Apply=%v before=%q after=%q", err, original, got)
			}
		})
	}
}

func TestUninstallRejectsProjectRootSwap(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	s := Scope{Project: root}
	commands := Commands{Native: testenv.NativeFixture(`"C:/tools/obit.exe" hook gemini`), Executable: testenv.NativeFixture(`C:\tools\obit.exe`)}
	plan, err := PlanGemini(s, commands, false)
	applyIntegration(t, plan, err)
	plan, err = PlanGemini(s, commands, true)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	for _, ch := range plan {
		if ch.existed {
			rel, err := filepath.Rel(root, ch.Path)
			if err != nil {
				t.Fatal(err)
			}
			writeIntegrationFile(t, filepath.Join(outside, rel), string(ch.orig))
		}
	}
	outsideRules := filepath.Join(outside, "GEMINI.md")
	original := read(t, outsideRules)
	if err := os.Rename(root, filepath.Join(parent, "original")); err != nil {
		t.Fatal(err)
	}
	makeIntegrationJunction(t, root, outside)
	t.Cleanup(func() { _ = os.Remove(root) })
	err = Apply(plan)
	got, readErr := os.ReadFile(outsideRules)
	if readErr != nil || string(got) != original || err == nil {
		t.Fatalf("uninstall removed other project's GEMINI.md: apply=%v read=%v content=%q", err, readErr, got)
	}
}

func TestApplyRejectsBackupLinkAfterPreflight(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	path := filepath.Join(root, "managed")
	writeIntegrationFile(t, path, "old")
	writeIntegrationFile(t, filepath.Join(outside, "sentinel"), "external")
	c, err := readIntegrationChange(Scope{Project: root}, path)
	if err != nil {
		t.Fatal(err)
	}
	c.Action, c.delete = "remove", true
	gate := filepath.Join(root, "gate")
	plan := []Change{{Path: gate, Action: "create", data: []byte("gate"), project: root, phase: -1}, c}
	backup := testBackupPath(t, path)
	if runtime.GOOS != "windows" {
		backup = path + ".bak" // POSIX removal preserves the entry beside its pinned parent.
	}
	if err := os.MkdirAll(filepath.Dir(backup), 0700); err != nil {
		t.Fatal(err)
	}
	afterIntegrationWrite := func(written string) error {
		if written == gate {
			makeIntegrationJunction(t, backup, outside)
		}
		return nil
	}
	t.Cleanup(func() { _ = os.Remove(backup) })
	err = apply(plan, afterIntegrationWrite)
	if runtime.GOOS != "windows" {
		if err != nil || read(t, backup+".1") != "old" || read(t, filepath.Join(outside, "sentinel")) != "external" {
			t.Fatalf("exclusive backup failed to preserve the original: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("verified original was not retired: %v", err)
		}
		if info, err := os.Lstat(backup); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("existing backup link was changed: %v", err)
		}
		return
	}
	if err == nil || read(t, path) != "old" || read(t, filepath.Join(outside, "sentinel")) != "external" {
		t.Fatalf("backup link planted after preflight accepted: %v", err)
	}
	info, statErr := os.Lstat(backup)
	if statErr != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
		t.Fatalf("planted backup link was not preserved: %v", statErr)
	}
}

func TestApplyPinsMissingHostDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	testenv.FakeClaude(t)
	parent, outside := t.TempDir(), t.TempDir()
	dir := filepath.Join(parent, "missing-config")
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	plan, err := PlanMCP(Scope{Global: true}, "claude", testenv.NativeFixture("C:/tools/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	attempted, swapped := false, false
	beforeIntegrationWrite = func(string) error {
		attempted = true
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := os.Rename(dir, dir+"-original"); err != nil {
			return nil
		}
		makeIntegrationJunction(t, dir, outside)
		swapped = true
		return nil
	}
	t.Cleanup(func() {
		beforeIntegrationWrite = nil
		if swapped {
			_ = os.Remove(dir)
		}
	})
	err = Apply(plan)
	beforeIntegrationWrite = nil
	if !attempted || swapped {
		t.Fatalf("host configuration parent was not pinned: attempted=%v swapped=%v Apply=%v", attempted, swapped, err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, plan[0].Path), "obituary") {
		t.Fatal("host MCP registration missing")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("host wrote outside configuration: %v %v", entries, err)
	}
}

func TestInstallFailureReportsAppliedAwarenessAndHooks(t *testing.T) {
	dir, scope := setup(t)
	changes, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), false)
	if err != nil {
		t.Fatal(err)
	}
	reference := filepath.Join(dir, "CLAUDE.md")
	err = Apply(editAfterPreflight(t, changes, reference, "user edit\n"))
	if err == nil {
		t.Fatal("concurrent edit must fail")
	}
	for _, name := range []string{"OBIT.md", "settings.json"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("applied %s missing from error: %v", name, err)
		}
	}
	if strings.Contains(read(t, reference), "@OBIT.md") {
		t.Error("user's concurrent save overwritten")
	}
}

func TestApplyRechecksUnchangedDependency(t *testing.T) {
	for _, edit := range []bool{false, true} {
		t.Run(map[bool]string{false: "deleted", true: "edited"}[edit], func(t *testing.T) {
			dir, scope := setup(t)
			install(t, scope, false)
			ref := filepath.Join(dir, "CLAUDE.md")
			if err := os.WriteFile(ref, []byte("user rules\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			changes, err := PlanClaude(scope, testenv.NativeFixture(`"C:/tools/obit.exe" hook claude`), false)
			if err != nil {
				t.Fatal(err)
			}
			awareness := filepath.Join(dir, "OBIT.md")
			if edit {
				err = os.WriteFile(awareness, []byte("user replacement\n"), 0o600)
			} else {
				err = os.Remove(awareness)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(changes); err == nil {
				t.Error("changed unchanged dependency must reject the entire plan")
			}
			if got := read(t, ref); got != "user rules\n" {
				t.Errorf("reference written despite missing dependency: %q", got)
			}
			if _, err := os.Stat(ref + ".bak.1"); !os.IsNotExist(err) {
				t.Errorf("preflight failure created a backup: %v", err)
			}
		})
	}
}
