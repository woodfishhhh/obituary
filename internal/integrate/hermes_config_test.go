package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHermesConfigKeepsUserAddedContainerMembers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HERMES_HOME", root)
	s := Scope{Global: true}
	path := filepath.Join(root, "config.yaml")
	writeIntegrationFile(t, path, "model: mine\n")
	plan := func(uninstall bool) []Change {
		t.Helper()
		base, err := PlanHermes(s, ownershipCommands("hermes"), uninstall)
		if err != nil {
			t.Fatal(err)
		}
		ext, err := PlanHermesMCP(s, testenv.NativeFixture("C:/tools/obit.exe"), uninstall)
		if err != nil {
			t.Fatal(err)
		}
		combined, err := ComposeAgentChanges(append(base, ext...))
		if err != nil {
			t.Fatal(err)
		}
		return combined
	}
	applyIntegration(t, plan(false), nil)
	change, err := PlanConfigEdits(s, path, ConfigYAML, []ConfigEdit{
		{Path: []string{"plugins", "note"}, Value: []byte(`"user-note"`)},
		{Path: []string{"plugins", "enabled"}, Value: []byte(`["obituary","user-plugin"]`)},
		{Path: []string{"mcp_servers", "user"}, Value: []byte(`{"command":"user-server"}`)},
	})
	applyIntegration(t, []Change{change}, err)
	applyIntegration(t, plan(true), nil)
	got := read(t, path)
	for _, want := range []string{"model: mine", "user-note", "user-plugin", "user-server"} {
		if !strings.Contains(got, want) {
			t.Fatal("removed later user container member", want, got)
		}
	}
	if strings.Contains(got, "obituary") {
		t.Fatal("owned registration remained", got)
	}
}
