package integrate

import (
	"path/filepath"
	"testing"
)

func TestRooMCP(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, ".roo", "mcp.json")
	checkMCPPlanCycle(t, PlanRooMCP, s, path, ConfigJSON, []string{"mcpServers"}, "")
	if _, err := PlanRooMCP(Scope{Global: true}, `C:\tools\obit.exe`, false); err == nil {
		t.Fatal("global MCP claimed installed")
	}
}

func TestRooInstallation(t *testing.T) {
	for _, global := range []bool{false, true} {
		root := t.TempDir()
		t.Setenv("OBIT_USER_HOME", root)
		checkAgentPlanCycle(t, "roo", Scope{Global: global, Project: root}, "", filepath.Join(root, ".roo", "rules", "obituary.md"))
	}
	checkAgentPreflight(t, "roo", Scope{Project: t.TempDir()})
	if _, err := PlanRoo(Scope{Project: t.TempDir(), GuardRM: true}, Commands{}, false); err == nil {
		t.Fatal("rules advertised as guard")
	}
}

func TestRooSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "roo")
}

func TestRooSkillSharedOwners(t *testing.T) {
	checkSharedSkillOwners(t, Scope{Project: t.TempDir()}, "codex", "roo")
}
