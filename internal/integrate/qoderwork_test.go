package integrate

import (
	"os"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/hook"
)

func TestQoderWorkManualCapabilities(t *testing.T) {
	t.Parallel()
	agent, ok := LookupAgent("qoderwork")
	if !ok || agent.Capabilities.DeleteGuard.Level != hook.Limited || agent.Capabilities.MCP.Level != hook.Limited {
		t.Fatalf("registry: %+v", agent)
	}
	if _, ok := hook.Lookup("qoderwork"); ok {
		t.Fatal("unverified native hook registered")
	}
	for _, global := range []bool{false, true} {
		s := Scope{Project: t.TempDir(), Global: global}
		for _, uninstall := range []bool{false, true} {
			p, err := agent.Plan(s, Commands{}, uninstall)
			if err != nil || PreservedError(p) == nil || Apply(p) == nil {
				t.Fatal("manual instructions claimed installed", p, err)
			}
			if p, err := agent.PlanMCP(s, "C:/tools with spaces/obit.exe", uninstall); err == nil || len(p) != 0 || !strings.Contains(err.Error(), "MCP") {
				t.Fatal("guessed MCP location", p, err)
			}
		}
		s.GuardRM = true
		if _, err := agent.Plan(s, Commands{}, false); err == nil || !strings.Contains(err.Error(), "#35") {
			t.Fatal("wrapper dependency hidden", err)
		}
		entries, err := os.ReadDir(s.Project)
		if err != nil || len(entries) != 0 {
			t.Fatal("manual target wrote files", entries, err)
		}
	}
}

func TestQoderWorkSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "qoderwork")
}
