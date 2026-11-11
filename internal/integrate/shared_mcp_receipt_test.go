package integrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedMCPLegacyDomesticRefreshKeepsUnknownOwners(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, owner := range []string{"qoder", "codebuddy", "minimax-code"} {
		t.Run(owner, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, ".mcp.json")
			installLegacyMCP(t, s, path, owner)
			a, _ := LookupAgent(owner)
			for range 2 {
				plan, err := a.PlanMCP(s, "C:/tools/obit.exe", false)
				applyIntegration(t, plan, err)
			}
			if !strings.Contains(read(t, ManagedAssetReceiptPath(path+".obituary-owners")), legacyMCPOwner) {
				t.Fatal("domestic refresh lost unverified legacy owners")
			}
			plan, err := a.PlanMCP(s, "", true)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(plan); err == nil || !strings.Contains(read(t, path), `"obituary"`) {
				t.Fatal("legacy registration was not retained after domestic refresh", err)
			}
		})
	}
}

func TestSharedMCPReceiptRejectsFutureSchema(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, ".mcp.json")
	plan, err := PlanMCP(s, "claude", "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
	receipt := ManagedAssetReceiptPath(path + ".obituary-owners")
	writeIntegrationFile(t, receipt, strings.Replace(read(t, receipt), `"schema":2`, `"schema":3`, 1))
	original := read(t, path)
	if _, err := PlanMCP(s, "claude", "C:/tools/obit.exe", false); err == nil {
		t.Fatal("future receipt was adopted")
	}
	plan, err = PlanMCP(s, "claude", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil || read(t, path) != original {
		t.Fatal("future receipt allowed registration removal", err)
	}
}
