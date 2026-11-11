package integrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindsurfInstallation(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	checkAgentPlanCycle(t, "windsurf", s, "", filepath.Join(s.Project, ".windsurfrules"))
	checkAgentPreflight(t, "windsurf", Scope{Project: t.TempDir()})
	if _, err := PlanWindsurf(Scope{Global: true}, Commands{}, false); err == nil || !strings.Contains(err.Error(), "Global") {
		t.Fatalf("global manual instructions: %v", err)
	}
	s.GuardRM = true
	if _, err := PlanWindsurf(s, Commands{}, false); err == nil {
		t.Fatal("rules advertised as guard")
	}
}

func TestWindsurfSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "windsurf")
}
