package integrate

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func TestSharedApplyHoldsResourceLockThroughWrites(t *testing.T) {
	for _, kind := range []string{"mcp", "rules"} {
		t.Run(kind, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, ".mcp.json")
			planner := func(remove bool) ([]Change, error) { return PlanMCP(s, "claude", "C:/tools/obit.exe", remove) }
			if kind == "rules" {
				path = filepath.Join(s.Project, "AGENTS.md")
				planner = func(remove bool) ([]Change, error) { return planSharedAgentRules(s, path, "hermes", remove) }
			}
			for _, uninstall := range []bool{false, true} {
				plan, err := planner(uninstall)
				if err != nil {
					t.Fatal(err)
				}
				checks, before := 0, 0
				beforeIntegrationWrite = func(string) error {
					before++
					if before == 1 {
						return nil // scheduling gap before Apply acquires the resource locks
					}
					release, ok, err := ledger.TryLockExclusive(ManagedAssetReceiptPath(testSidecarPath(t, path+".obituary-owners")) + ".lock")
					if ok {
						release()
						t.Fatal("shared resource lock released before receipt commit")
					}
					return err
				}
				afterWrite := func(string) error {
					checks++
					release, ok, err := ledger.TryLockExclusive(ManagedAssetReceiptPath(testSidecarPath(t, path+".obituary-owners")) + ".lock")
					if ok {
						release()
						t.Fatal("shared resource lock released before receipt commit")
					}
					return err
				}
				t.Cleanup(func() { beforeIntegrationWrite = nil })
				if err := apply(plan, afterWrite); err != nil {
					t.Fatal(err)
				}
				beforeIntegrationWrite = nil
				if uninstall && before < 4 {
					t.Fatalf("did not exercise all transaction removals: %d", before)
				}
				if !uninstall && checks < 3 {
					t.Fatalf("did not exercise all transaction writes: %d", checks)
				}
				release, ok, err := ledger.TryLockExclusive(ManagedAssetReceiptPath(testSidecarPath(t, path+".obituary-owners")) + ".lock")
				if err != nil || !ok {
					t.Fatalf("resource lock leaked after Apply: %v", err)
				}
				release()
			}
		})
	}
}

func TestSharedApplyRejectsInstallerDuringTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, kind := range []string{"mcp", "rules"} {
		t.Run(kind, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, ".mcp.json")
			var planner func(string) ([]Change, error)
			owner, other := "claude", "copilot"
			if kind == "rules" {
				path = filepath.Join(s.Project, "AGENTS.md")
				owner, other = "hermes", "kimi"
				planner = func(owner string) ([]Change, error) { return planSharedAgentRules(s, path, owner, false) }
			} else {
				planner = func(owner string) ([]Change, error) {
					a, _ := LookupAgent(owner)
					return a.PlanMCP(s, "C:/tools/obit.exe", false)
				}
			}
			plan, err := planner(owner)
			if err != nil {
				t.Fatal(err)
			}
			attempts := 0
			afterWrite := func(written string) error {
				if written != path || attempts > 0 {
					return nil
				}
				attempts++
				competing, err := planner(other)
				if err != nil {
					return err
				}
				if err := Apply(competing); err == nil || !strings.Contains(err.Error(), "busy") {
					t.Fatalf("concurrent installer bypassed shared resource lock: %v", err)
				}
				return nil
			}
			if err := apply(plan, afterWrite); err != nil {
				t.Fatal(err)
			}
			if attempts != 1 {
				t.Fatal("concurrent installer did not run")
			}
			plan, err = planner(other)
			applyIntegration(t, plan, err)
		})
	}
}
