package integrate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedSkillOwners(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "user"}[global], func(t *testing.T) {
			root := agentGoldenEnvironment(t)
			t.Setenv("GEMINI_CLI_HOME", filepath.Join(root, "OBIT_USER_HOME"))
			names := []string{"codex", "cursor", "copilot", "droid", "gemini", "kimi", "cline", "kilocode", "opencode", "pi", "omp", "roo"}
			if !global {
				names = append(names, "antigravity", "hermes", "openclaw", "trae-cn")
			}
			checkSharedSkillOwners(t, Scope{Global: global, Project: filepath.Join(root, "project")}, names...)
		})
	}
}

func TestSharedSkillLegacyCodexOwner(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	path, _ := skillPath(s, "codex")
	writeIntegrationFile(t, path, string(skillContent))
	writeIntegrationFile(t, filepath.Join(filepath.Dir(path), ".obituary-skill"), string(skillReceipt(skillContent)))
	plan, err := PlanSharedSkill(s, "pi", false)
	applyIntegration(t, plan, err)
	plan, err = PlanSkill(s, "codex", true)
	applyIntegration(t, plan, err)
	if read(t, path) != string(skillContent) {
		t.Fatal("legacy Codex removal deleted Pi skill")
	}
	plan, err = PlanSharedSkill(s, "pi", true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("last owner did not remove legacy skill", err)
	}
}

func TestSharedSkillConcurrentOwners(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	first, err := PlanSharedSkill(s, "pi", false)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := PlanSharedSkill(s, "roo", false)
	applyIntegration(t, first, err)
	if err := Apply(stale); err == nil {
		t.Fatal("stale owner plan accepted")
	}
	fresh, err := PlanSharedSkill(s, "roo", false)
	applyIntegration(t, fresh, err)
	plan, err := PlanSharedSkill(s, "pi", true)
	applyIntegration(t, plan, err)
	if read(t, first[0].Path) != string(skillContent) {
		t.Fatal("lost owner")
	}
}

func TestSharedSkillInterruptedReceipt(t *testing.T) {
	for _, step := range []string{".obituary-skill", "SKILL.md"} {
		t.Run(step, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			plan, err := PlanSharedSkill(s, "pi", false)
			if err != nil {
				t.Fatal(err)
			}
			stop := errors.New("interrupted")
			err = apply(plan, func(path string) error {
				if filepath.Base(path) == step {
					return stop
				}
				return nil
			})
			if !errors.Is(err, stop) {
				t.Fatal("interruption not reached", err)
			}
			plan, err = PlanSharedSkill(s, "pi", false)
			applyIntegration(t, plan, err)
			if read(t, plan[0].Path) != string(skillContent) {
				t.Fatal("interrupted install did not recover")
			}
		})
	}
}

func TestSharedSkillOrphanReceiptStatus(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	plan, err := PlanSharedSkill(s, "pi", false)
	applyIntegration(t, plan, err)
	if err := os.Remove(plan[0].Path); err != nil {
		t.Fatal(err)
	}
	status, err := InspectExtension(s, "pi", "skill", "obit")
	if err != nil || status.Installed {
		t.Fatalf("orphan receipt is not a skill: %+v %v", status, err)
	}
}

func TestSharedSkillPlanShowsDestination(t *testing.T) {
	plan, err := PlanSharedSkill(Scope{Project: t.TempDir()}, "default", false)
	if err != nil || !strings.Contains(plan[0].Detail, "Agent Skill → "+plan[0].Path) {
		t.Fatal("missing skill destination", plan, err)
	}
}

func TestSharedSkillUpgradeRetainsOwners(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	root := filepath.Join(s.Project, ".agents", "skills")
	for _, name := range []string{"pi", "roo"} {
		plan, err := planSkillAsset(s, root, sharedSkillID, name, []byte("previous skill"), false)
		applyIntegration(t, plan, err)
	}
	plan, err := PlanSharedSkill(s, "pi", false)
	applyIntegration(t, plan, err)
	plan, err = PlanSharedSkill(s, "pi", true)
	applyIntegration(t, plan, err)
	path := filepath.Join(root, "obituary", "SKILL.md")
	if read(t, path) != string(skillContent) {
		t.Fatal("shared upgrade lost payload or owner")
	}
	plan, err = PlanSharedSkill(s, "roo", true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("last owner retained upgraded skill", err)
	}
}

func TestSharedSkillMigratesOwnedHostCopy(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	legacyRoot := filepath.Join(s.Project, ".cursor", "skills")
	path := filepath.Join(legacyRoot, "obituary", "SKILL.md")
	legacy, err := PlanManagedAssets(s, []ManagedAsset{{Path: path, ID: "cursor-obituary-skill", Owner: "cursor", Version: "1", Content: []byte("old embedded skill")}}, false)
	applyIntegration(t, legacy, err)
	plan, err := PlanCursorSkill(s, false)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("old copy retained", err)
	}
	if _, err := os.Stat(ManagedAssetReceiptPath(path)); !os.IsNotExist(err) {
		t.Fatal("old receipt retained", err)
	}
	if read(t, filepath.Join(s.Project, ".agents", "skills", "obituary", "SKILL.md")) != string(skillContent) {
		t.Fatal("missing shared copy")
	}
}

func TestSharedSkillMigrationPreservesEditedHostCopy(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, ".cursor", "skills", "obituary", "SKILL.md")
	writeIntegrationFile(t, path, "user skill")
	if _, err := PlanCursorSkill(s, false); err == nil {
		t.Fatal("unowned previous copy accepted")
	}
	shared := filepath.Join(s.Project, ".agents", "skills", "obituary", "SKILL.md")
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatal("conflicting shared copy written", err)
	}
	plan, err := PlanCursorSkill(s, true)
	applyPreservedSkill(t, plan, err)
	if read(t, path) != "user skill" {
		t.Fatal("previous copy lost")
	}
}

func TestSharedSkillMigrationChecksAbsentLegacyPath(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	plan, err := PlanCursorSkill(s, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Project, ".cursor", "skills", "obituary", "SKILL.md")
	writeIntegrationFile(t, path, "concurrent user skill")
	if err := Apply(plan); err == nil {
		t.Fatal("ignored concurrent legacy copy")
	}
	if read(t, path) != "concurrent user skill" {
		t.Fatal("concurrent edit lost")
	}
}

func TestSharedSkillRejectsEmptyOwner(t *testing.T) {
	if _, err := PlanSharedSkill(Scope{Project: t.TempDir()}, "", false); err == nil {
		t.Fatal("empty owner accepted")
	}
}
