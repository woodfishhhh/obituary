package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKimiGlobalHookLifecycle(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KIMI_SHARE_DIR", root)
	path := filepath.Join(root, "config.toml")
	input := "\xef\xbb\xbf# user settings\nmodel='keep'\n[[hooks]]\nevent='Notification'\ncommand='echo user' # user comment\n"
	writeIntegrationFile(t, path, input)
	s, commands := Scope{Global: true}, Commands{Native: `"C:/space & tools/obit.exe" hook kimi`}
	plan, err := PlanKimi(s, commands, false)
	if err != nil || read(t, path) != input {
		t.Fatalf("dry-run: %v", err)
	}
	applyIntegration(t, plan, nil)
	got := read(t, path)
	for _, event := range []string{"PreToolUse", "PostToolUse", "SessionStart"} {
		if strings.Contains(got, event) {
			t.Fatal("unsupported callback registered", event)
		}
	}
	if !strings.Contains(got, "SessionEnd") || !strings.Contains(got, "Stop") {
		t.Fatal("missing silent end events")
	}
	writeIntegrationFile(t, path, strings.Replace(got, "# obit:kimi-hooks:begin", "# obit:kimi-hooks:begin\n# owned comment", 1))
	commands.Native = `"D:/new location/obit.exe" hook kimi`
	plan, err = PlanKimi(s, commands, false)
	applyIntegration(t, plan, err)
	got = read(t, path)
	if !strings.HasPrefix(got, input) || !strings.Contains(got, "# owned comment") || !strings.Contains(got, "new location") || strings.Contains(got, "space & tools") {
		t.Fatal("refresh damaged config", got)
	}
	plan, err = PlanKimi(s, commands, false)
	if err != nil || Pending(plan) {
		t.Fatal("not idempotent", err)
	}
	plan, err = PlanKimi(s, Commands{}, true)
	applyIntegration(t, plan, err)
	if strings.TrimSpace(read(t, path)) != strings.TrimSpace(input) || read(t, path+".bak") != input {
		t.Fatal("user config lost")
	}
	plan, err = PlanKimi(s, Commands{}, true)
	if err != nil || Pending(plan) {
		t.Fatal("repeat uninstall", err)
	}
}

func TestKimiProjectRulesLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	input := "\xef\xbb\xbf# User rules\nkeep these\n"
	writeIntegrationFile(t, path, input)
	plan, err := PlanKimi(s, Commands{}, false)
	if err != nil || read(t, path) != input || !Pending(plan) {
		t.Fatal("rules dry-run", err)
	}
	applyIntegration(t, plan, nil)
	plan, err = PlanKimi(s, Commands{}, false)
	if err != nil || Pending(plan) {
		t.Fatal("rules not idempotent", err)
	}
	if _, err := os.Stat(filepath.Join(s.Project, ".kimi", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("invented project auto-loaded config")
	}
	plan, err = PlanKimi(s, Commands{}, true)
	applyIntegration(t, plan, err)
	if read(t, path) != input {
		t.Fatal("user rules changed", read(t, path))
	}
}

func TestKimiRejectsUnsafeConfig(t *testing.T) {
	for _, input := range []string{"[", "hooks={}", "[[hooks]]\nevent='Stop'\ncommand='C:/tools/obit.exe hook kimi'\n", "# obit:kimi-hooks:begin\n[[hooks]]\nevent='Stop'\ncommand='user'\n# obit:kimi-hooks:end\n"} {
		root := t.TempDir()
		t.Setenv("KIMI_SHARE_DIR", root)
		path := filepath.Join(root, "config.toml")
		writeIntegrationFile(t, path, input)
		for _, uninstall := range []bool{false, true} {
			if _, err := PlanKimi(Scope{Global: true}, Commands{Native: "C:/tools/obit.exe hook kimi"}, uninstall); err == nil {
				t.Fatal("invalid config accepted", input)
			}
		}
		if read(t, path) != input {
			t.Fatal("planner wrote config")
		}
	}
}

func TestKimiSkillDiscoveryAndOwnership(t *testing.T) {
	for _, global := range []bool{false, true} {
		home, redirected := t.TempDir(), t.TempDir()
		t.Setenv("OBIT_USER_HOME", home)
		t.Setenv("KIMI_SHARE_DIR", redirected)
		s := Scope{Project: t.TempDir(), Global: global}
		root := s.Project
		if global {
			root = home
		}
		path := filepath.Join(root, ".kimi", "skills", "obituary", "SKILL.md")
		plan, err := PlanKimiSkill(s, false)
		applyIntegration(t, plan, err)
		if read(t, path) != string(skillContent) {
			t.Fatal("skill differs")
		}
		plan, err = PlanKimiSkill(s, false)
		if err != nil || Pending(plan) {
			t.Fatal("skill not idempotent", err)
		}
		if _, err := os.Stat(filepath.Join(redirected, "skills")); !os.IsNotExist(err) {
			t.Fatal("skills followed runtime data override")
		}
		plan, err = PlanKimiSkill(s, true)
		applyIntegration(t, plan, err)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("owned skill remains")
		}
		plan, err = PlanKimiSkill(s, false)
		applyIntegration(t, plan, err)
		writeIntegrationFile(t, path, "edited skill")
		if _, err := PlanKimiSkill(s, false); err == nil {
			t.Fatal("edited skill overwritten")
		}
		plan, err = PlanKimiSkill(s, true)
		if err != nil || Apply(plan) == nil || read(t, path) != "edited skill" {
			t.Fatal("edited skill not preserved", err)
		}
	}
}
