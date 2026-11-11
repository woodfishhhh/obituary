package integrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeRuleChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, shared := range []bool{false, true} {
		s := Scope{Project: t.TempDir()}
		path := filepath.Join(s.Project, "AGENTS.md")
		original := "user instructions\r\n"
		writeIntegrationFile(t, path, original)
		for _, remove := range []bool{false, false, true} {
			var plans []Change
			for _, name := range []string{"pi", "omp"} {
				var ch Change
				var err error
				if shared {
					ch, err = planAgentRule(s, path, remove)
				} else {
					ch, err = planPluginRules(s, path, name, remove)
				}
				if err != nil {
					t.Fatal(err)
				}
				plans = append(plans, ch)
			}
			joined, err := ComposeAgentChanges(plans)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(joined); err != nil {
				t.Fatal(err)
			}
			data := read(t, path)
			if !strings.HasPrefix(data, original) || remove && strings.Contains(data, "obit:") {
				t.Fatalf("lost instructions or removal: %s", data)
			}
		}
	}
}

func TestComposeRuleChangesRejectsOverlapAndRace(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, "AGENTS.md")
	a, err := planPluginRules(s, path, "pi", false)
	if err != nil {
		t.Fatal(err)
	}
	b := a
	a.existed, b.existed = true, true
	a.orig, b.orig = []byte("old"), []byte("old")
	a.data, b.data = []byte("one"), []byte("two")
	if _, err := ComposeAgentChanges([]Change{a, b}); err == nil {
		t.Fatal("overlapping rule replacements accepted")
	}
	a, _ = planPluginRules(s, path, "pi", false)
	b, _ = planPluginRules(s, path, "omp", false)
	joined, err := ComposeAgentChanges([]Change{a, b})
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, path, "concurrent user edit")
	if err := Apply(joined); err == nil || read(t, path) != "concurrent user edit" {
		t.Fatal("composed rules lost concurrent edit", err)
	}
}
