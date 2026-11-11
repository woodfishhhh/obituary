package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLingmaInstallerLifecycle(t *testing.T) {
	for _, global := range []bool{false, true} {
		s, dir := qoderTestScope(t, "lingma", global)
		path := filepath.Join(dir, "settings.json")
		writeIntegrationFile(t, path, qoderUserConfig)
		p, err := PlanLingma(s, qoderTestCommands("lingma", "tools"), false)
		if err != nil || read(t, path) != qoderUserConfig {
			t.Fatal("dry-run", err)
		}
		err = Apply(p)
		if global && err == nil || !global && err != nil {
			t.Fatal("manual global rules must remain visible", err)
		}
		p, err = PlanLingma(s, qoderTestCommands("lingma", "tools"), false)
		if err != nil || Pending(p) {
			t.Fatal("idempotence", err, p)
		}
		p, err = PlanLingma(s, qoderTestCommands("lingma", "new tools"), false)
		if err != nil {
			t.Fatal(err)
		}
		_ = Apply(p) // global manual-rule preservation is expected above
		text := read(t, path)
		if strings.Count(text, "hook lingma") != 2 || strings.Contains(text, "SessionStart") || strings.Contains(text, "SessionEnd") || !strings.Contains(text, "new tools") {
			t.Fatal("wrong five-event subset", text)
		}
		p, err = PlanLingma(s, Commands{}, true)
		if err != nil {
			t.Fatal(err)
		}
		_ = Apply(p)
		if strings.Contains(read(t, path), "hook lingma") || !strings.Contains(read(t, path), "user-lint") {
			t.Fatal("uninstall", read(t, path))
		}
	}
}

func TestLingmaMCPManualAndIndependentRoots(t *testing.T) {
	s, dir := qoderTestScope(t, "lingma", false)
	for _, uninstall := range []bool{false, true} {
		if p, err := PlanLingmaMCP(s, "C:/tools/obit.exe", uninstall); err == nil || len(p) != 0 || !strings.Contains(err.Error(), "MCP") {
			t.Fatal("unverified MCP pretended installed", p, err)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("manual MCP wrote files", err)
	}
	p, err := PlanLingma(s, qoderTestCommands("lingma", "tools"), false)
	applyIntegration(t, p, err)
	if _, err := os.Stat(filepath.Join(s.Project, ".qoder")); !os.IsNotExist(err) {
		t.Fatal("Lingma used CN project root", err)
	}
}

func TestLingmaSkillTargets(t *testing.T) {
	checkAgentSkillTargets(t, "lingma")
}
