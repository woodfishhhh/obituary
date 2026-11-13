package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallKeepsTheUsersSettingsAndIsIdempotent(t *testing.T) {
	dir, scope := setup(t)
	install(t, scope, false)

	settings := read(t, filepath.Join(dir, "settings.json"))
	var parsed struct {
		Theme string                       `json:"theme"`
		Model string                       `json:"model"`
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(settings, "\xef\xbb\xbf")), &parsed); err != nil {
		t.Fatalf("settings.json is no longer valid JSON: %v\n%s", err, settings)
	}
	if parsed.Theme != "dark" || parsed.Model != "opus" || !strings.Contains(settings, "my-linter") {
		t.Fatalf("the user's settings were lost:\n%s", settings)
	}
	if strings.Index(settings, `"theme"`) > strings.Index(settings, `"hooks"`) || strings.Index(settings, `"hooks"`) > strings.Index(settings, `"model"`) {
		t.Fatalf("key order changed:\n%s", settings)
	}
	for _, ev := range []string{"SessionStart", "PostToolUse", "Stop", "SessionEnd"} {
		var groups []struct{ Hooks []handler }
		if err := json.Unmarshal(mustJSON(parsed.Hooks[ev]), &groups); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, g := range groups {
			for _, h := range g.Hooks {
				found = found || OwnedHookExec(h.Command, h.Args, "claude", nil)
			}
		}
		if !found {
			t.Errorf("%s exec hook missing", ev)
		}
	}
	if len(parsed.Hooks["PostToolUse"]) != 2 {
		t.Fatalf("want the user's group plus ours, got %d", len(parsed.Hooks["PostToolUse"]))
	}
	if read(t, filepath.Join(dir, "settings.json.bak")) != userSettings {
		t.Fatal("the original settings.json must be backed up")
	}
	if md := read(t, filepath.Join(dir, "CLAUDE.md")); !strings.HasPrefix(md, "# My rules") || strings.Count(md, "@OBIT.md") != 1 {
		t.Fatalf("CLAUDE.md: %q", md)
	}

	again, err := PlanClaude(scope, testenv.NativeFixture(`"C:/tools/obit.exe" hook claude`), false)
	if err != nil || Pending(again) {
		t.Fatalf("a second install must change nothing, got %+v %v", again, err)
	}
}

func TestClaudePowerShellGuardRefresh(t *testing.T) {
	dir, scope := setup(t)
	path := filepath.Join(dir, "settings.json")
	old := testenv.NativeFixture(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"C:/tools/obit.exe hook claude"}]},{"matcher":"PowerShell","hooks":[{"type":"command","command":"user-hook"}]}]}}`)
	writeIntegrationFile(t, path, old)
	changes, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), false)
	if err != nil {
		t.Fatal(err)
	}
	registered, covered := GuardRMStatus(changes, "claude")
	if !registered || covered {
		t.Fatal("old Bash-only guard must need refresh")
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if !strings.Contains(got, `"matcher": ".*"`) || !strings.Contains(got, "user-hook") {
		t.Fatalf("PowerShell matcher missing or user hook lost: %s", got)
	}
	again, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), false)
	if err != nil || Pending(again) || !GuardRMInstalled(again, "claude") {
		t.Fatalf("refresh not current: %+v %v", again, err)
	}
	if read(t, path+".bak") != old {
		t.Fatal("old settings not backed up")
	}
}

func TestClaudeExecMigratesLegacyAndPreservesForeignArguments(t *testing.T) {
	dir, scope := setup(t)
	path := filepath.Join(dir, "settings.json")
	foreign := testenv.NativeFixture(`{"type":"command","command":"C:/tools/obit.exe hook claude","args":["--help"]}`)
	original := testenv.NativeFixture(`{"hooks":{"PreToolUse":[{"matcher":"Bash|PowerShell","hooks":[{"type":"command","command":"C:/old/obit.exe hook claude"},`) + foreign + `]}]}}`
	writeIntegrationFile(t, path, original)
	plan, err := PlanClaudeExec(scope, testenv.NativeFixture("C:/new folder/obit.exe"), false)
	if err != nil {
		t.Fatal(err)
	}
	if !GuardRMInstalled(plan, "claude") || read(t, path) != original {
		t.Fatal("legacy inspection changed configuration")
	}
	applyIntegration(t, plan, nil)
	if strings.Contains(read(t, path), testenv.NativeFixture("C:/old")) || !strings.Contains(read(t, path), "--help") {
		t.Fatal("migration or ownership failed")
	}
	plan, err = PlanClaudeExec(scope, testenv.NativeFixture("C:/new folder/obit.exe"), false)
	if err != nil || Pending(plan) || !GuardRMInstalled(plan, "claude") {
		t.Fatalf("exec installation not current: %+v %v", plan, err)
	}
	plan, err = PlanClaudeExec(scope, testenv.NativeFixture("C:/new folder/obit.exe"), true)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), "--help") || strings.Contains(read(t, path), testenv.NativeFixture("C:/new")) {
		t.Fatal("foreign argv removed")
	}
	plan, err = PlanClaudeExec(scope, testenv.NativeFixture("C:/new folder/obit.exe"), true)
	if err != nil || GuardRMInstalled(plan, "claude") {
		t.Fatal("foreign argv reported as installed guard")
	}
}

func TestUninstallRemovesOnlyObituary(t *testing.T) {
	dir, scope := setup(t)
	install(t, scope, false)
	install(t, scope, true)

	settings := read(t, filepath.Join(dir, "settings.json"))
	if strings.Contains(settings, "hook claude") || !strings.Contains(settings, "my-linter") ||
		!strings.Contains(settings, `"theme"`) || strings.Contains(settings, "SessionStart") {
		t.Fatalf("uninstall must remove exactly what was added:\n%s", settings)
	}
	if _, err := os.Stat(filepath.Join(dir, "OBIT.md")); !os.IsNotExist(err) {
		t.Fatal("OBIT.md must be removed")
	}
	if md := read(t, filepath.Join(dir, "CLAUDE.md")); strings.Contains(md, "@OBIT.md") || !strings.Contains(md, "Be brief.") {
		t.Fatalf("CLAUDE.md: %q", md)
	}
}

func TestBrokenSettingsAreLeftUntouched(t *testing.T) {
	dir, scope := setup(t)
	broken := `{"hooks": [`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), false); err == nil {
		t.Fatal("invalid JSON must be refused")
	}
	if read(t, filepath.Join(dir, "settings.json")) != broken {
		t.Fatal("the file must not be modified")
	}
}

func TestReinstallRefreshesAStaleCommandWithoutDuplicates(t *testing.T) {
	dir, scope := setup(t)
	install(t, scope, false)
	changes, err := PlanClaude(scope, testenv.NativeFixture(`"D:/moved/obit.exe" hook claude`), false)
	if err != nil || !Pending(changes) {
		t.Fatalf("a moved binary must be noticed, got %+v %v", changes, err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	settings := read(t, filepath.Join(dir, "settings.json"))
	if strings.Contains(settings, testenv.NativeFixture("C:/tools/obit.exe")) || strings.Count(settings, testenv.NativeFixture("D:/moved/obit.exe")) != 5 {
		t.Fatalf("want exactly the five refreshed handlers:\n%s", settings)
	}
}

func TestAUsersCompoundCommandIsNotOurs(t *testing.T) {
	dir, scope := setup(t)
	compound := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"my-linter && obit hook claude"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(compound), 0o600); err != nil {
		t.Fatal(err)
	}
	install(t, scope, false)
	install(t, scope, true)
	if settings := read(t, filepath.Join(dir, "settings.json")); !strings.Contains(settings, "my-linter && obit hook claude") {
		t.Fatalf("the user's own command must survive uninstall:\n%s", settings)
	}
}

func TestMultilineUserHooksSurviveInstallAndUninstall(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, agent := range []string{"claude", "codex"} {
		for _, newline := range []string{"\n", "\r", "\r\n"} {
			t.Run(fmt.Sprintf("%s/%q", agent, newline), func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("CLAUDE_CONFIG_DIR", dir)
				t.Setenv("CODEX_HOME", dir)
				command := "notify-me" + newline + "/mnt/d/tools/obit.exe hook " + agent
				path := filepath.Join(dir, "settings.json")
				plan := func(uninstall bool) ([]Change, error) {
					return PlanClaude(Scope{Global: true}, testenv.NativeFixture("C:/tools/obit.exe hook claude"), uninstall)
				}
				if agent == "codex" {
					path = filepath.Join(dir, "hooks.json")
					plan = func(uninstall bool) ([]Change, error) {
						return PlanCodex(Scope{Global: true}, unixCmd, winCmd, uninstall)
					}
				}
				data := fmt.Sprintf(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":%s}]}]}}`, mustJSON(command))
				if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
				for _, uninstall := range []bool{false, false, true} {
					changes, err := plan(uninstall)
					if err != nil {
						t.Fatal(err)
					}
					if err := Apply(changes); err != nil {
						t.Fatal(err)
					}
					var parsed struct {
						Hooks map[string][]struct {
							Hooks []handler `json:"hooks"`
						} `json:"hooks"`
					}
					if err := json.Unmarshal([]byte(read(t, path)), &parsed); err != nil {
						t.Fatal(err)
					}
					found := false
					for _, g := range parsed.Hooks["Stop"] {
						for _, h := range g.Hooks {
							found = found || h.Command == command
						}
					}
					if !found {
						t.Fatalf("user command lost (uninstall=%v): %q", uninstall, command)
					}
				}
			})
		}
	}
}

func TestAForeignOBITmdIsNeverReplaced(t *testing.T) {
	dir, scope := setup(t)
	mine := "# my own notes called OBIT.md\n"
	if err := os.WriteFile(filepath.Join(dir, "OBIT.md"), []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), false); err == nil {
		t.Fatal("installing over someone else's OBIT.md must be refused")
	}
	if changes, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), true); err != nil || Apply(changes) != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if read(t, filepath.Join(dir, "OBIT.md")) != mine {
		t.Fatal("uninstall must not delete a file Obituary does not own")
	}
}

func TestTheOriginalBackupSurvivesReinstalls(t *testing.T) {
	dir, scope := setup(t)
	install(t, scope, false)
	install(t, scope, true)
	install(t, scope, false)
	if read(t, filepath.Join(dir, "settings.json.bak")) != userSettings {
		t.Fatal("settings.json.bak must still hold the pre-install original")
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json.bak.1")); err != nil {
		t.Fatal("later backups go to numbered files")
	}
}

func TestApplyRefusesAFileChangedAfterPlanning(t *testing.T) {
	dir, scope := setup(t)
	changes, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), false)
	if err != nil {
		t.Fatal(err)
	}
	edited := `{"theme":"light"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err == nil {
		t.Fatal("a file saved after planning must not be overwritten")
	}
	if read(t, filepath.Join(dir, "settings.json")) != edited {
		t.Fatal("the user's edit was lost")
	}
}

func TestApplyRechecksBeforeReplace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	orig, edited := []byte(`{"theme":"dark"}`), []byte(`{"theme":"light"}`)
	if err := os.WriteFile(path, orig, 0o600); err != nil {
		t.Fatal(err)
	}
	// The first write makes a deterministic edit after the shared preflight.
	// The second change must not overwrite it using its stale original.
	changes := []Change{
		{Path: path, Action: "update", existed: true, orig: orig, data: edited},
		{Path: path, Action: "update", existed: true, orig: orig, data: []byte(`{"hooks":{}}`)},
	}
	if err := Apply(changes); err == nil {
		t.Error("a change after preflight must be refused")
	}
	if got := read(t, path); got != string(edited) {
		t.Errorf("edit overwritten: %s", got)
	}
	if temps, err := filepath.Glob(filepath.Join(dir, ".obit-*.tmp")); err != nil || len(temps) != 0 {
		t.Errorf("temporary file leaked: %v, %v", temps, err)
	}
}

func TestTrailingDataAfterTheJSONIsRejected(t *testing.T) {
	dir, scope := setup(t)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"a":1}{"b":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), false); err == nil {
		t.Fatal("two concatenated objects are not a settings file to rewrite")
	}
}

func TestClaudeFailureKeepsAwarenessDependencies(t *testing.T) {
	for _, uninstall := range []bool{false, true} {
		t.Run(map[bool]string{false: "install", true: "uninstall"}[uninstall], func(t *testing.T) {
			dir, scope := setup(t)
			if uninstall {
				install(t, scope, false)
			}
			changes, err := PlanClaude(scope, testenv.NativeFixture("C:/tools/obit.exe hook claude"), uninstall)
			if err != nil {
				t.Fatal(err)
			}
			awareness, reference := filepath.Join(dir, "OBIT.md"), filepath.Join(dir, "CLAUDE.md")
			path, edit := awareness, "user saved this file\n"
			if uninstall {
				path, edit = reference, read(t, reference)+"user edit\n"
			}
			err = Apply(editAfterPreflight(t, changes, path, edit))
			if err == nil {
				t.Fatal("concurrent edit must fail")
			}
			if !strings.Contains(err.Error(), "already applied") || !strings.Contains(err.Error(), path) {
				t.Errorf("partial changes missing from error: %v", err)
			}
			if read(t, path) != edit {
				t.Error("user edit overwritten")
			}
			if uninstall {
				if !strings.Contains(read(t, reference), "@OBIT.md") {
					t.Error("user reference edit lost")
				}
				if _, err := os.Stat(awareness); err != nil {
					t.Errorf("reference points to absent OBIT.md: %v", err)
				}
			} else {
				if strings.Contains(read(t, filepath.Join(dir, "settings.json")), "hook claude") {
					t.Error("hooks installed before awareness was ready")
				}
				if strings.Contains(read(t, reference), "@OBIT.md") {
					t.Error("reference installed before awareness was ready")
				}
			}
		})
	}
}
