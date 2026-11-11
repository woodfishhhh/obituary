package integrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookConfigBOMLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	const bom = "\xef\xbb\xbf"
	for _, agent := range []string{"claude", "codex"} {
		for _, tc := range []struct {
			name, prefix string
			exists, user bool
		}{
			{"without-bom", "", true, true},
			{"with-bom", bom, true, true},
			{"new-file", "", false, false},
			{"only-owned-without-bom", "", true, false},
			{"only-owned-with-bom", bom, true, false},
		} {
			t.Run(agent+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("CLAUDE_CONFIG_DIR", dir)
				t.Setenv("CODEX_HOME", dir)
				path := filepath.Join(dir, "settings.json")
				if agent == "codex" {
					path = filepath.Join(dir, "hooks.json")
				}
				original := "{}"
				userCommand := "echo user && obit hook " + agent
				if tc.user {
					original = `{"theme":"深色<&>","hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` + userCommand + `"}]}]},"model":"keep"}`
				}
				if tc.exists {
					writeIntegrationFile(t, path, tc.prefix+original)
				}
				var installed string
				for _, step := range []struct {
					name, binary string
					uninstall    bool
					unchanged    bool
				}{
					{"install", "C:/old/obit.exe", false, false},
					{"repeat-install", "C:/old/obit.exe", false, true},
					{"refresh", "C:/new/obit.exe", false, false},
					{"repeat-refresh", "C:/new/obit.exe", false, true},
					{"uninstall", "C:/new/obit.exe", true, false},
					{"repeat-uninstall", "C:/new/obit.exe", true, true},
				} {
					before, err := os.ReadFile(path)
					if err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					info, err := os.Stat(path)
					if err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					backupsBefore, err := filepath.Glob(path + ".bak*")
					if err != nil {
						t.Fatal(err)
					}
					command := step.binary + " hook " + agent
					scope := Scope{Global: true, GuardRM: true}
					var changes []Change
					if agent == "claude" {
						changes, err = PlanClaude(scope, command, step.uninstall)
					} else {
						changes, err = PlanCodex(scope, command, command, step.uninstall)
					}
					if err != nil {
						t.Fatalf("%s: %v", step.name, err)
					}
					if Pending(changes) != !step.unchanged {
						t.Fatalf("%s: plan=%+v", step.name, changes)
					}
					if err := Apply(changes); err != nil {
						t.Fatalf("%s: %v", step.name, err)
					}
					backupsAfter, err := filepath.Glob(path + ".bak*")
					wantBackups := len(backupsBefore)
					if !step.unchanged && info != nil {
						if got := read(t, backupPath(path, wantBackups)); got != string(before) {
							t.Fatalf("%s: backup changed original bytes: %q", step.name, got)
						}
						wantBackups++
					}
					if err != nil || len(backupsAfter) != wantBackups {
						t.Fatalf("%s: backups=%v want count=%d err=%v", step.name, backupsAfter, wantBackups, err)
					}
					if step.uninstall && !tc.user {
						if _, err := os.Stat(path); !os.IsNotExist(err) {
							t.Fatalf("%s: owned-only hook file remains: %v", step.name, err)
						}
						continue
					}
					got := read(t, path)
					if strings.HasPrefix(got, bom) != (tc.prefix != "") {
						t.Fatalf("%s: BOM changed: %q", step.name, got)
					}
					content := strings.TrimPrefix(got, bom)
					var root map[string]json.RawMessage
					if err := json.Unmarshal([]byte(content), &root); err != nil {
						t.Fatalf("%s: invalid JSON: %v", step.name, err)
					}
					if tc.user {
						if string(root["theme"]) != `"深色<&>"` || string(root["model"]) != `"keep"` || !strings.Contains(content, userCommand) ||
							strings.Index(content, `"theme"`) > strings.Index(content, `"hooks"`) || strings.Index(content, `"hooks"`) > strings.Index(content, `"model"`) {
							t.Fatalf("%s: user content or key order changed: %s", step.name, got)
						}
					}
					if step.unchanged {
						after, err := os.Stat(path)
						if err != nil || string(before) != got || info == nil || !os.SameFile(info, after) {
							t.Fatalf("%s: unchanged file rewritten: %v", step.name, err)
						}
					}
					switch step.name {
					case "install":
						installed = got
						if !strings.Contains(got, step.binary) {
							t.Fatal("hooks were not installed")
						}
						var compact bytes.Buffer
						if err := json.Compact(&compact, []byte(content)); err != nil {
							t.Fatal(err)
						}
						// A no-op install also preserves the user's compact formatting.
						writeIntegrationFile(t, path, tc.prefix+compact.String())
					case "refresh":
						if want := strings.ReplaceAll(installed, "C:/old/obit.exe", step.binary); got != want {
							t.Fatalf("refresh changed more than the hook command: %q", got)
						}
					case "uninstall", "repeat-uninstall":
						if !sameJSON([]byte(content), []byte(original)) {
							t.Fatalf("%s: user configuration changed: %s", step.name, got)
						}
					}
				}
			})
		}
	}
}

func TestGuardRMStatusDecodedArguments(t *testing.T) {
	for _, tc := range []struct {
		name, agent, handler string
		guard                bool
	}{
		{"escaped-args", "claude", `{"type":"command","command":"C:/tools/obit.exe","args":["hook","claude","\u002d\u002dnotice-only"]}`, false},
		{"escaped-command", "codex", `{"type":"command","command":"C:/tools/obit.exe hook codex \u002d\u002dnotice-only"}`, false},
		{"directory", "claude", `{"type":"command","command":"C:/--notice-only/obit.exe hook claude"}`, true},
		{"metadata", "codex", `{"type":"command","command":"C:/tools/obit.exe hook codex","description":"--notice-only"}`, true},
		{"mixed-modes", "codex", `{"type":"command","command":"/mnt/c/tools/obit.exe hook codex --notice-only","commandWindows":"C:/tools/obit.exe hook codex"}`, false},
		{"mixed-modes-escaped", "codex", `{"type":"command","command":"/mnt/c/tools/obit.exe hook codex","commandWindows":"C:/tools/obit.exe hook codex \u002d\u002dnotice-only"}`, false},
		{"both-guard", "codex", `{"type":"command","command":"/mnt/c/tools/obit.exe hook codex","commandWindows":"C:/tools/obit.exe hook codex"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := `{"hooks":{"PreToolUse":[{"matcher":".*","hooks":[` + tc.handler + `]}]}}`
			registered, effective := GuardRMStatus([]Change{{orig: []byte(data)}}, tc.agent)
			if registered != tc.guard || effective != tc.guard {
				t.Errorf("guard registered=%t effective=%t want=%t", registered, effective, tc.guard)
			}
			path := filepath.Join(t.TempDir(), "hooks.json")
			writeIntegrationFile(t, path, data)
			command, err := graveNoticeCommand(Scope{Global: true}, path, tc.agent, "C:/new/obit.exe hook "+tc.agent)
			if err != nil || strings.HasSuffix(command, " --notice-only") == tc.guard {
				t.Errorf("refresh changed guard opt-in: %q %v", command, err)
			}
		})
	}
}

func TestGuardRMInstalledWithBOM(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "codex"} {
		for _, bom := range []string{"", "\xef\xbb\xbf"} {
			data := bom + `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"C:/tools/obit.exe hook ` + agent + `"}]}]}}`
			if !GuardRMInstalled([]Change{{orig: []byte(data)}}, agent) {
				t.Errorf("installed %s guard missed (BOM %t)", agent, bom != "")
			}
		}
	}
}

func TestGuardRMMatcherCoverage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		agent, matcher string
		want           bool
	}{
		{"claude", "Bash", false}, {"claude", "^Bash$", false},
		{"claude", "Bash|PowerShell", true}, {"claude", "^(Bash|PowerShell)$", true},
		{"claude", "Write|Edit", false}, {"claude", "BashOther", false},
		{"claude", "[", false}, {"claude", "*", true}, {"claude", "", true},
		{"codex", "Bash", false}, {"codex", "exec_command|shell_command", false},
		{"codex", "Bash|exec_command|shell_command|shell|local_shell", true},
		{"codex", "^(Bash|exec_command|shell_command|shell|local_shell)$", true},
		{"codex", ".*", true}, {"codex", "apply_patch", false},
	} {
		t.Run(tc.agent+"/"+tc.matcher, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"hooks": map[string]any{"PreToolUse": []any{map[string]any{"matcher": tc.matcher, "hooks": []handler{{Type: "command", Command: "C:/tools/obit.exe hook " + tc.agent}}}}}})
			changes := []Change{{orig: data}}
			registered, effective := GuardRMStatus(changes, tc.agent)
			if !registered || effective != tc.want || GuardRMInstalled(changes, tc.agent) != tc.want {
				t.Fatalf("registered=%v effective=%v want=%v", registered, effective, tc.want)
			}
		})
	}
	data := `{"hooks":{"PreToolUse":[{"matcher":"Bash|exec_command","hooks":[{"type":"command","command":"C:/tools/obit.exe hook codex"}]},{"matcher":"shell_command|shell|local_shell","hooks":[{"type":"command","command":"C:/tools/obit.exe hook codex"}]}]}}`
	if !GuardRMInstalled([]Change{{orig: []byte(data)}}, "codex") {
		t.Fatal("split matcher coverage missed")
	}
	for _, data := range []string{
		`{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"C:/tools/obit.exe hook claude"}]}]}}`,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo user"}]}]}}`,
	} {
		if registered, effective := GuardRMStatus([]Change{{orig: []byte(data)}}, "claude"); registered || effective {
			t.Fatalf("foreign registration counted: %s", data)
		}
	}
}

func TestGuardPlanOwnershipBackupsAndConflict(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", dir)
			t.Setenv("CODEX_HOME", dir)
			path := filepath.Join(dir, "settings.json")
			if agent == "codex" {
				path = filepath.Join(dir, "hooks.json")
			}
			user := `{"theme":"dark","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo user && obit hook ` + agent + `"}]}]},"model":"keep"}`
			if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
				t.Fatal(err)
			}
			plan := func(uninstall bool) []Change {
				scope := Scope{Global: true, GuardRM: true}
				var changes []Change
				var err error
				if agent == "codex" {
					changes, err = PlanCodex(scope, "C:/tools/obit.exe hook codex", "C:/tools/obit.exe hook codex", uninstall)
				} else {
					changes, err = PlanClaude(scope, "C:/tools/obit.exe hook claude", uninstall)
				}
				if err != nil {
					t.Fatal(err)
				}
				return changes
			}
			changes := plan(false)
			if err := os.WriteFile(path, []byte(user+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Apply(changes); err == nil {
				t.Fatal("changed source was overwritten")
			}
			if err := Apply(plan(false)); err != nil {
				t.Fatal(err)
			}
			if got := read(t, path+".bak"); got != user+"\n" {
				t.Fatalf("backup lost: %q", got)
			}
			installed := read(t, path)
			if !strings.Contains(installed, "echo user && obit hook "+agent) || strings.Index(installed, `"theme"`) > strings.Index(installed, `"hooks"`) || strings.Index(installed, `"hooks"`) > strings.Index(installed, `"model"`) {
				t.Fatalf("user config altered: %s", installed)
			}
			var root struct {
				Hooks map[string][]struct {
					Matcher string    `json:"matcher"`
					Hooks   []handler `json:"hooks"`
				} `json:"hooks"`
			}
			if err := json.Unmarshal([]byte(installed), &root); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, group := range root.Hooks["PreToolUse"] {
				for _, h := range group.Hooks {
					if h.Command == "C:/tools/obit.exe hook "+agent || OwnedHookExec(h.Command, h.Args, agent, nil) {
						found = true
						if group.Matcher != ".*" {
							t.Fatal("wrong matcher", group.Matcher)
						}
					}
				}
			}
			if !found {
				t.Fatal("no guard registration")
			}
			if Pending(plan(false)) {
				t.Fatal("not idempotent")
			}
			if err := Apply(plan(true)); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(read(t, path), "echo user && obit hook "+agent) {
				t.Fatal("uninstall removed foreign handler")
			}
			if got := read(t, path+".bak"); got != user+"\n" {
				t.Fatal("uninstall overwrote first backup")
			}
			if read(t, path+".bak.1") != installed {
				t.Fatal("numbered backup missing")
			}
		})
	}
}
