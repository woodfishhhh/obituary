package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestVibeInstallationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{true: "global", false: "project"}[global], func(t *testing.T) {
			t.Setenv("VIBE_HOME", filepath.Join(t.TempDir(), "vibe"))
			s := Scope{Project: t.TempDir(), Global: global}
			root, _ := vibeRoot(s)
			path := filepath.Join(root, "hooks.toml")
			input := "\xef\xbb\xbf# user header\n[[hooks]]\nname = 'user'\ntype = 'post_tool'\ncommand = 'echo user' # command comment\n"
			writeIntegrationFile(t, path, input)
			if !global {
				writeIntegrationFile(t, filepath.Join(s.Project, "AGENTS.md"), "# Project rules\nkeep these\n")
			}
			commands := Commands{Native: testenv.NativeFixture(`"C:/tools/obit.exe" hook vibe`)}
			plan, err := PlanVibe(s, commands, false)
			if err != nil || read(t, path) != input {
				t.Fatalf("dry-run: %v", err)
			}
			applyIntegration(t, plan, nil)
			got := read(t, path)
			var parsed map[string]any
			if err := toml.Unmarshal(bytes.TrimPrefix([]byte(got), []byte("\xef\xbb\xbf")), &parsed); err != nil {
				t.Fatal(err)
			}
			entries := parsed["hooks"].([]any)
			if len(entries) != 3 || entries[1].(map[string]any)["type"] != "post_tool" || entries[2].(map[string]any)["type"] != "post_agent" {
				t.Fatal("unsupported or missing event", entries)
			}
			writeIntegrationFile(t, path, strings.Replace(got, "# obit:vibe-hooks:begin", "# obit:vibe-hooks:begin\n# retained owned comment", 1))
			commands.Native = testenv.NativeFixture("D:/new/obit.exe hook vibe")
			plan, err = PlanVibe(s, commands, false)
			applyIntegration(t, plan, err)
			got = read(t, path)
			if !strings.HasPrefix(got, input) || !strings.Contains(got, "# retained owned comment") || strings.Contains(got, testenv.NativeFixture("C:/tools")) || strings.Count(got, testenv.NativeFixture("D:/new/obit.exe hook vibe")) != 2 {
				t.Fatal("refresh did not preserve comments", got)
			}
			plan, err = PlanVibe(s, commands, false)
			if err != nil || Pending(plan) {
				t.Fatal("not idempotent", err)
			}
			if global {
				if _, err := os.Stat(filepath.Join(root, "config.toml")); !os.IsNotExist(err) {
					t.Fatal("selected user's prompt implicitly")
				}
				found := false
				for _, ch := range plan {
					found = found || strings.Contains(ch.Detail, "system_prompt_id")
				}
				if !found {
					t.Fatal("missing prompt activation instructions")
				}
			}
			plan, err = PlanVibe(s, Commands{}, true)
			applyIntegration(t, plan, err)
			if strings.TrimSpace(read(t, path)) != strings.TrimSpace(input) || read(t, testBackupPath(t, path)) != input {
				t.Fatal("user hooks lost")
			}
			if !global && !strings.Contains(read(t, filepath.Join(s.Project, "AGENTS.md")), "keep these") {
				t.Fatal("project rules lost")
			}
			plan, err = PlanVibe(s, Commands{}, true)
			if err != nil || Pending(plan) {
				t.Fatal("repeat uninstall", err)
			}
		})
	}
}

func TestVibeRejectsUnmarkedHooksAndUnsafeLauncher(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, input := range []string{"[", "hooks = {}", "[[hooks]]\nname='obituary-write'\ncommand='user'\n", testenv.NativeFixture("[[hooks]]\ncommand='C:/tools/obit.exe hook vibe'\n"), "# obit:vibe-hooks:begin\n[[hooks]]\ncommand='user'\n# obit:vibe-hooks:end\n"} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, ".vibe", "hooks.toml")
		writeIntegrationFile(t, path, input)
		for _, uninstall := range []bool{false, true} {
			if _, err := PlanVibe(s, Commands{Native: testenv.NativeFixture("C:/tools/obit.exe hook vibe")}, uninstall); err == nil {
				t.Fatal("unowned or invalid hook accepted", input)
			}
		}
		if read(t, path) != input {
			t.Fatal("planner wrote config")
		}
	}
	for _, command := range []string{testenv.NativeFixture(`"C:/space dir/obit.exe" hook vibe`), testenv.NativeFixture(`"C:/tools&stuff/obit.exe" hook vibe`), "echo user && obit hook vibe", testenv.NativeFixture("& 'C:/tools/obit.exe' hook vibe")} {
		if _, err := PlanVibe(Scope{Project: t.TempDir()}, Commands{Native: command}, false); err == nil {
			t.Fatal("ambiguous shell accepted", command)
		}
	}
}

func TestVibeSkillLifecycleAndEditedPrompt(t *testing.T) {
	for _, global := range []bool{false, true} {
		s := Scope{Project: t.TempDir(), Global: global}
		t.Setenv("VIBE_HOME", t.TempDir())
		root, _ := vibeRoot(s)
		path := filepath.Join(root, "skills", "obituary", "SKILL.md")
		plan, err := PlanVibeSkill(s, false)
		applyIntegration(t, plan, err)
		if read(t, path) != string(skillContent) {
			t.Fatal("embedded skill changed")
		}
		plan, err = PlanVibeSkill(s, false)
		if err != nil || Pending(plan) {
			t.Fatal("skill not idempotent", err)
		}
		plan, err = PlanVibeSkill(s, true)
		applyIntegration(t, plan, err)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("skill not removed")
		}
	}
	t.Setenv("VIBE_HOME", t.TempDir())
	s := Scope{Global: true}
	plan, err := PlanVibe(s, Commands{Native: testenv.NativeFixture("C:/tools/obit.exe hook vibe")}, false)
	applyIntegration(t, plan, err)
	root, _ := vibeRoot(s)
	path := filepath.Join(root, "prompts", "obituary.md")
	writeIntegrationFile(t, path, "user prompt")
	plan, err = PlanVibe(s, Commands{}, true)
	if err != nil || Apply(plan) == nil || read(t, path) != "user prompt" {
		t.Fatal("edited prompt not preserved", err)
	}
}
