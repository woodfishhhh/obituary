package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedAgentRulesLifecycle(t *testing.T) {
	t.Parallel()
	for _, together := range []bool{false, true} {
		t.Run(map[bool]string{false: "sequential", true: "together"}[together], func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, "AGENTS.md")
			user := "\xef\xbb\xbf# My policy\r\nKeep this.  \r\n"
			writeIntegrationFile(t, path, user)
			agents := []string{"codex", "hermes", "kimi", "vibe"}
			var plans []Change
			for _, agent := range agents {
				plan, err := PlanAgent(agent, s, ownershipCommands(agent), false)
				if err != nil {
					t.Fatal(err)
				}
				if together {
					plans = append(plans, plan...)
				} else {
					applyIntegration(t, plan, nil)
				}
			}
			if together {
				plan, err := ComposeAgentChanges(plans)
				applyIntegration(t, plan, err)
			}
			installed := read(t, path)
			for i, agent := range agents {
				plan, err := PlanAgent(agent, s, ownershipCommands(agent), true)
				applyIntegration(t, plan, err)
				want := installed
				if i == len(agents)-1 {
					want = user
				}
				if got := read(t, path); got != want {
					t.Fatalf("shared rules changed before last owner %s: %q", agent, got)
				}
				plan, err = PlanAgent(agent, s, ownershipCommands(agent), true)
				if err != nil || Pending(plan) {
					t.Fatalf("repeat uninstall changed owners: %v", err)
				}
			}
		})
	}
}

func TestSharedAgentRulesLegacyOwnersRemainProtected(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	legacy := blockBegin + "\nold shared instructions\n" + blockEnd + "\n"
	writeIntegrationFile(t, path, legacy)
	writeIntegrationFile(t, filepath.Join(s.Project, ".codex", "hooks.json"), "\xef\xbb\xbf"+`{"hooks":{"Stop":[{"hooks":[{"type":"command","commandWindows":"C:/tools/obit.exe hook codex"}]}]}}`)
	plan, err := PlanAgent("hermes", s, ownershipCommands("hermes"), true)
	if err != nil {
		t.Fatal(err)
	}
	if Apply(plan) == nil || read(t, path) != legacy {
		t.Fatal("unverified legacy block was removed or reported removed")
	}
	plan, err = PlanAgent("hermes", s, ownershipCommands("hermes"), false)
	applyIntegration(t, plan, err)
	for _, agent := range []string{"hermes", "codex"} {
		plan, err = PlanAgent(agent, s, ownershipCommands(agent), true)
		applyIntegration(t, plan, err)
		if !strings.Contains(read(t, path), blockBegin) {
			t.Fatal("migration discarded an unknown legacy owner")
		}
	}
	if _, err := os.Stat(path + ".obituary-owners"); err != nil {
		t.Fatal("legacy owner protection was not persisted", err)
	}
}

func TestSharedAgentRulesUninstallRechecksConcurrentOwners(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	plan, err := planSharedAgentRules(s, path, "hermes", false)
	applyIntegration(t, plan, err)
	plan, err = planSharedAgentRules(s, path, "hermes", true)
	if err != nil {
		t.Fatal(err)
	}
	beforeIntegrationWrite = func(writing string) error {
		if writing != path {
			return nil
		}
		beforeIntegrationWrite = nil
		other, err := planSharedAgentRules(s, path, "kimi", false)
		if err != nil {
			return err
		}
		return Apply(other)
	}
	t.Cleanup(func() { beforeIntegrationWrite = nil })
	err = Apply(plan)
	data, readErr := os.ReadFile(path)
	if readErr != nil || !strings.Contains(string(data), blockBegin) {
		t.Fatalf("concurrent owner rules were deleted: %v; read=%v", err, readErr)
	}
	if err == nil {
		t.Fatal("stale owner receipt was accepted")
	}
	plan, err = planSharedAgentRules(s, path, "hermes", true)
	applyIntegration(t, plan, err)
	plan, err = planSharedAgentRules(s, path, "kimi", true)
	applyIntegration(t, plan, err)
}

func TestSharedAgentRulesPreserveOtherOwners(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"hermes", "kimi", "vibe"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			plan, err := PlanAgent("codex", s, ownershipCommands("codex"), false)
			applyIntegration(t, plan, err)
			path := filepath.Join(s.Project, "AGENTS.md")
			original := read(t, path)
			plan, err = PlanAgent(agent, s, ownershipCommands(agent), true)
			applyIntegration(t, plan, err)
			got, err := os.ReadFile(path)
			if err != nil || string(got) != original {
				t.Fatalf("uninstalling never-installed %s removed Codex rules: err=%v content=%q", agent, err, got)
			}
		})
	}
}
