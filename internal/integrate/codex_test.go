package integrate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexHookOwnershipLifecycle(t *testing.T) {
	for _, uninstall := range []bool{true, false} {
		for _, suffix := range []string{"", " --notice-only"} {
			for _, tc := range []struct {
				name, command, windows string
				owned, mixed           bool
			}{
				{"relative", "./tools/obit.exe", "", false, false},
				{"bare", "obit.exe", "", false, false},
				{"mixed-windows", "/mnt/c/old/obit.exe", "./tools/obit.exe", false, true},
				{"mixed-posix", "user-tool", "C:/old/obit.exe", false, true},
				{"absolute", "C:/old/obit.exe", "", true, false},
				{"wsl", "/mnt/c/old/obit.exe", "", true, false},
				{"both", "/mnt/c/old/obit.exe", "C:/old/obit.exe", true, false},
				{"windows-only", "", "C:/old/obit.exe", true, false},
			} {
				t.Run(fmt.Sprintf("uninstall=%t/%s/%s", uninstall, suffix, tc.name), func(t *testing.T) {
					h := codexHandler{Type: "command"}
					if tc.command != "" {
						h.Command = tc.command + " hook codex" + suffix
					}
					if tc.windows != "" {
						h.CommandWindows = tc.windows + " hook codex" + suffix
					}
					testCodexHookOwnership(t, h, uninstall, tc.owned, tc.mixed)
				})
			}
		}
	}
}

func testCodexHookOwnership(t *testing.T, h codexHandler, uninstall, owned, mixed bool) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	path := filepath.Join(dir, "hooks.json")
	raw, _ := json.Marshal(h)
	original := `{"user":"keep","hooks":{"PreToolUse":[{"matcher":".*","hooks":[` + string(raw) + `]}]}}`
	writeIntegrationFile(t, path, original)
	changes, err := PlanCodex(Scope{Global: true}, unixCmd, winCmd, uninstall)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	var root struct {
		Hooks map[string][]struct{ Hooks []json.RawMessage } `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(got), &root); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, group := range root.Hooks["PreToolUse"] {
		for _, handler := range group.Hooks {
			found = found || sameJSON(handler, raw)
		}
	}
	if found == owned {
		t.Errorf("handler preservation=%t owned=%t: %s", found, owned, got)
	}
	if uninstall && !owned && got != original {
		t.Error("uninstall rewrote a foreign handler")
	}
	if mixed && !strings.Contains(changes[0].Detail, "保留：含非讣告命令") {
		t.Errorf("mixed handler lacks preservation note: %+v", changes[0])
	}
}

func TestCodexInstallServesWindowsAndWSLAndUninstallsCleanly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	userHooks := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify-me"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), []byte(userHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Global rules\n\nReply in Chinese.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Global: true}
	changes, err := PlanCodex(scope, unixCmd, winCmd, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}

	hooks := read(t, filepath.Join(dir, "hooks.json"))
	var parsed struct {
		Hooks map[string][]struct {
			Hooks []codexHandler `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(hooks), &parsed); err != nil {
		t.Fatal(err)
	}
	stop := parsed.Hooks["Stop"]
	if len(stop) != 2 || stop[0].Hooks[0].Command != "notify-me" || stop[1].Hooks[0].Command != unixCmd || stop[1].Hooks[0].CommandWindows != winCmd {
		t.Fatalf("want the user's Stop hook and ours with both commands:\n%s", hooks)
	}
	if end := parsed.Hooks["SessionEnd"]; len(end) != 1 || end[0].Hooks[0].Timeout != 3 {
		t.Fatalf("SessionEnd needs its 3 s timeout:\n%s", hooks)
	}
	agents := read(t, filepath.Join(dir, "AGENTS.md"))
	if !strings.HasPrefix(agents, "# Global rules") || strings.Count(agents, blockBegin) != 1 || !strings.Contains(agents, "obit mktemp") {
		t.Fatalf("AGENTS.md:\n%s", agents)
	}
	if again, _ := PlanCodex(scope, unixCmd, winCmd, false); Pending(again) {
		t.Fatalf("a second install must change nothing: %+v", again)
	}

	changes, err = PlanCodex(scope, unixCmd, winCmd, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	if hooks := read(t, filepath.Join(dir, "hooks.json")); strings.Contains(hooks, "hook codex") || !strings.Contains(hooks, "notify-me") {
		t.Fatalf("uninstall must leave only the user's hook:\n%s", hooks)
	}
	if agents := read(t, filepath.Join(dir, "AGENTS.md")); strings.Contains(agents, "obit:") || !strings.Contains(agents, "Reply in Chinese.") {
		t.Fatalf("AGENTS.md after uninstall:\n%s", agents)
	}
}

func TestBlockPreservesNonStandaloneMarkerVariants(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for agent, relative := range map[string]string{"gemini": "GEMINI.md", "copilot": ".github/copilot-instructions.md", "droid": ".factory/AGENTS.md", "codebuddy": "CODEBUDDY.md"} {
		for _, mode := range []string{"install", "refresh", "uninstall", "never-installed-uninstall"} {
			t.Run(agent+"/"+mode, func(t *testing.T) {
				s := Scope{Project: t.TempDir()}
				if mode == "refresh" || mode == "uninstall" {
					plan, err := PlanAgent(agent, s, ownershipCommands(agent), false)
					applyIntegration(t, plan, err)
				}
				for _, eol := range []string{"\n", "\r\n"} {
					for _, bom := range []string{"", "\xef\xbb\xbf"} {
						for _, variant := range []string{"begin-prefix", "begin-suffix", "end-prefix", "end-suffix"} {
							t.Run(fmt.Sprintf("%q/%q/%s", eol, bom, variant), func(t *testing.T) {
								testBlockMarkerVariant(t, s, agent, relative, mode, eol, bom, variant)
							})
						}
					}
				}
			})
		}
	}
}

func TestBlockPreservesStandaloneMarkerBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for agent, relative := range map[string]string{"gemini": "GEMINI.md", "copilot": ".github/copilot-instructions.md", "droid": ".factory/AGENTS.md", "codebuddy": "CODEBUDDY.md"} {
		for _, eol := range []string{"\n", "\r\n"} {
			for _, bom := range []string{"", "\xef\xbb\xbf"} {
				t.Run(fmt.Sprintf("%s/%q/%q", agent, eol, bom), func(t *testing.T) {
					s := Scope{Project: t.TempDir()}
					path := filepath.Join(s.Project, filepath.FromSlash(relative))
					prefix := bom + "User policy  " + eol + eol
					suffix := eol + "KEEP MY POLICY  " + eol
					writeIntegrationFile(t, path, prefix)
					plan, err := PlanAgent(agent, s, ownershipCommands(agent), false)
					applyIntegration(t, plan, err)
					writeIntegrationFile(t, path, prefix+blockBegin+eol+"old instructions"+eol+blockEnd+eol+suffix)
					plan, err = PlanAgent(agent, s, ownershipCommands(agent), false)
					applyIntegration(t, plan, err)
					got := read(t, path)
					if !strings.HasPrefix(got, prefix+blockBegin+eol) || !strings.HasSuffix(got, blockEnd+eol+suffix) || strings.Contains(got, "old instructions") {
						t.Fatalf("refresh changed standalone block surroundings: %q", got)
					}
					plan, err = PlanAgent(agent, s, ownershipCommands(agent), true)
					applyIntegration(t, plan, err)
					if got := read(t, path); got != prefix+suffix {
						t.Fatalf("uninstall changed standalone block surroundings: %q", got)
					}
				})
			}
		}
	}
}

func testBlockMarkerVariant(t *testing.T, s Scope, agent, relative, mode, eol, bom, variant string) {
	t.Helper()
	begin, end := blockBegin, blockEnd
	switch variant {
	case "begin-prefix":
		begin = "example: " + begin
	case "begin-suffix":
		begin += " is an example"
	case "end-prefix":
		end = "example: " + end
	case "end-suffix":
		end += " is an example"
	}
	original := bom + strings.Join([]string{"User policy", begin, "KEEP MY POLICY", end, ""}, eol)
	path := filepath.Join(s.Project, filepath.FromSlash(relative))
	writeIntegrationFile(t, path, original)
	plan, err := PlanAgent(agent, s, ownershipCommands(agent), strings.Contains(mode, "uninstall"))
	if err == nil {
		err = Apply(plan)
	}
	if got := read(t, path); got != original {
		t.Fatalf("non-standalone block changed user policy: %q; err=%v", got, err)
	}
	if err == nil {
		t.Fatal("non-standalone markers accepted")
	}
}

func TestProjectInstallRefusesALinkedConfigDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, ".codex"), outside).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	if _, err := PlanCodex(Scope{Project: root}, unixCmd, winCmd, false); err == nil {
		t.Fatal("a .codex junction pointing outside the project must be refused")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("nothing may be written outside the project")
	}
}

func TestApplyRechecksProjectLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, uninstall := range []bool{false, true} {
		t.Run(fmt.Sprint(uninstall), func(t *testing.T) {
			t.Parallel()
			root, outside := t.TempDir(), t.TempDir()
			scope := Scope{Project: root}
			if uninstall {
				changes, err := PlanCodex(scope, unixCmd, winCmd, false)
				if err != nil {
					t.Fatal(err)
				}
				if err := Apply(changes); err != nil {
					t.Fatal(err)
				}
			}
			changes, err := PlanCodex(scope, unixCmd, winCmd, uninstall)
			if err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(root, ".codex")
			if uninstall {
				data := read(t, filepath.Join(config, "hooks.json"))
				if err := os.WriteFile(filepath.Join(outside, "hooks.json"), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(config, config+".saved"); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", config, outside).CombinedOutput(); err != nil {
				t.Fatalf("mklink: %v %s", err, out)
			}
			if err := Apply(changes); err == nil {
				t.Error("Apply must reject a junction added after planning")
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if uninstall {
				if len(entries) != 1 || entries[0].Name() != "hooks.json" || read(t, filepath.Join(outside, "hooks.json")) != read(t, filepath.Join(config+".saved", "hooks.json")) {
					t.Errorf("outside configuration changed: %v", entries)
				}
			} else if len(entries) != 0 {
				t.Errorf("outside directory received writes: %v", entries)
			}
		})
	}
}

func TestApplyRechecksNumberedBackupLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root, outside := t.TempDir(), t.TempDir()
	scope := Scope{Project: root}
	changes, err := PlanCodex(scope, unixCmd, winCmd, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	changes, err = PlanCodex(scope, unixCmd, winCmd, true)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".codex", "hooks.json")
	orig := read(t, path)
	backup := testBackupPath(t, path)
	writeIntegrationFile(t, backup, "earlier backup")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", backup+".1", outside).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	if err := Apply(changes); err == nil {
		t.Error("numbered backup link added after planning must be refused")
	}
	if got := read(t, path); got != orig {
		t.Error("preflight failure changed hooks")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Errorf("outside directory changed: %v, %v", entries, err)
	}
}

func TestCodexUninstallRemovesBlockBeforeHooks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir := t.TempDir()
	scope := Scope{Project: dir}
	changes, err := PlanCodex(scope, unixCmd, winCmd, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	changes, err = PlanCodex(scope, unixCmd, winCmd, true)
	if err != nil {
		t.Fatal(err)
	}
	ref := filepath.Join(dir, "AGENTS.md")
	err = Apply(editAfterPreflight(t, changes, ref, read(t, ref)+"user edit\n"))
	if err == nil {
		t.Fatal("concurrent edit must fail")
	}
	if !strings.Contains(read(t, ref), blockBegin) {
		t.Error("user edit lost")
	}
	if data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json")); err != nil || !strings.Contains(string(data), "hook codex") {
		t.Errorf("hooks removed before block: %s, %v", data, err)
	}
}
