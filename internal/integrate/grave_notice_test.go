package integrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexMixedNoticeModesRequireGuardOptIn(t *testing.T) {
	for _, guard := range []bool{false, true} {
		for _, suffixes := range [][2]string{{" --notice-only", ""}, {"", " \\u002d\\u002dnotice-only"}} {
			dir := t.TempDir()
			t.Setenv("CODEX_HOME", dir)
			path := filepath.Join(dir, "hooks.json")
			writeIntegrationFile(t, path, `{"hooks":{"PreToolUse":[{"matcher":".*","hooks":[{"type":"command","command":"/mnt/c/old/obit.exe hook codex`+suffixes[0]+`","commandWindows":"C:/old/obit.exe hook codex`+suffixes[1]+`"}]}]}}`)
			s := Scope{Global: true, GuardRM: guard}
			changes, err := PlanCodex(s, unixCmd, winCmd, false)
			applyIntegration(t, changes, err)
			changes, err = PlanCodex(s, unixCmd, winCmd, false)
			if err != nil || Pending(changes) || GuardRMInstalled(changes, "codex") != guard {
				t.Fatalf("mixed modes changed opt-in=%t: %+v %v", guard, changes, err)
			}
			want := 2
			if guard {
				want = 0
			}
			if got := strings.Count(read(t, path), "--notice-only"); got != want {
				t.Fatalf("platform notice flags=%d want=%d", got, want)
			}
		}
	}
}

func TestGraveNoticeInstallationPreservesGuardOptIn(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", dir)
			t.Setenv("CODEX_HOME", dir)
			plan := func(guard, remove bool) []Change {
				t.Helper()
				s := Scope{Global: true, GuardRM: guard}
				var changes []Change
				var err error
				if agent == "claude" {
					changes, err = PlanClaude(s, "C:/tools/obit.exe hook claude", remove)
				} else {
					changes, err = PlanCodex(s, "C:/tools/obit.exe hook codex", "C:/tools/obit.exe hook codex", remove)
				}
				if err != nil {
					t.Fatal(err)
				}
				return changes
			}
			if err := Apply(plan(false, false)); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "settings.json")
			if agent == "codex" {
				path = filepath.Join(dir, "hooks.json")
			}
			out := read(t, path)
			if !strings.Contains(out, "--notice-only") || !strings.Contains(out, `"PreToolUse"`) || GuardRMInstalled(plan(false, false), agent) {
				t.Fatal(out)
			}
			if Pending(plan(false, false)) {
				t.Fatal("advisory refresh not idempotent")
			}
			if err := Apply(plan(true, false)); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(read(t, path), "--notice-only") || !GuardRMInstalled(plan(false, false), agent) {
				t.Fatal("guard opt-in not retained")
			}
			if err := Apply(plan(false, true)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
