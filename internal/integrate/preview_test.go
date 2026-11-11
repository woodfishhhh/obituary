package integrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyPlanCannotApply(t *testing.T) {
	root := t.TempDir()
	s := (Scope{Project: root}).ForInspection()
	plan, err := PlanMCP(s, "codex", "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil || !strings.Contains(err.Error(), "read-only installation plan") {
		t.Fatalf("read-only plan was applied: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview changed directory: %v %v", entries, err)
	}
	s = Scope{Project: root}
	install, err := PlanMCP(s, "codex", "C:/tools/obit.exe", false)
	if err != nil {
		t.Fatal(err)
	}
	previewJSON, _ := json.Marshal(plan)
	installJSON, _ := json.Marshal(install)
	if string(previewJSON) != string(installJSON) {
		t.Fatalf("preview differs from install: %s vs %s", previewJSON, installJSON)
	}
	combined, err := ComposeAgentChanges(append(install, plan...))
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(combined); err == nil || !strings.Contains(err.Error(), "read-only installation plan") {
		t.Fatalf("composed read-only plan was applied: %v", err)
	}
	applyIntegration(t, install, nil)
}

func TestReadOnlyPlanRejectsNonDirectoryParent(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "config")
	writeIntegrationFile(t, parent, "user file")
	if _, err := readIntegrationChange((Scope{Global: true}).ForInspection(), filepath.Join(parent, "managed")); err == nil {
		t.Fatal("preview treated a file parent as a missing directory")
	}
}

func TestReadOnlyPlanRejectsLinkedAncestors(t *testing.T) {
	if testing.Short() {
		t.Skip("creates real junctions with a child process")
	}
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "missing"}[missing], func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			writeIntegrationFile(t, filepath.Join(outside, "managed"), "external")
			link := filepath.Join(root, "linked")
			makeIntegrationJunction(t, link, outside)
			path := filepath.Join(link, "managed")
			if missing {
				path = filepath.Join(link, "missing", "managed")
			}
			if _, err := readIntegrationChange((Scope{Global: true}).ForInspection(), path); err == nil {
				t.Fatal("preview followed linked ancestor")
			}
		})
	}
}

func TestInspectionObservesChangesOnNextInvocation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "managed")
	writeIntegrationFile(t, path, "original")
	s := (Scope{Global: true}).ForInspection()
	first, err := readIntegrationChange(s, path)
	if err != nil || string(first.orig) != "original" {
		t.Fatalf("first inspection: %q %v", first.orig, err)
	}
	writeIntegrationFile(t, path, "edited")
	next, err := readIntegrationChange((Scope{Global: true}).ForInspection(), path)
	if err != nil || string(next.orig) != "edited" {
		t.Fatalf("next inspection missed edit: %q %v", next.orig, err)
	}
	if err := os.Rename(root, root+"-after"); err != nil {
		t.Fatalf("inspection retained handles: %v", err)
	}
	t.Cleanup(func() { _ = os.Rename(root+"-after", root) })
}
