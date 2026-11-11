package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedMCPOwnersLifecycle(t *testing.T) {
	t.Parallel()
	for _, together := range []bool{false, true} {
		t.Run(map[bool]string{false: "sequential", true: "together"}[together], func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, ".mcp.json")
			agents := []string{"claude", "codebuddy", "qoder", "minimax-code", "copilot"}
			var plans []Change
			for _, agent := range agents {
				a, _ := LookupAgent(agent)
				plan, err := a.PlanMCP(s, "C:/tools/obit.exe", false)
				if err != nil {
					t.Fatal(err)
				}
				if together {
					plans = append(plans, plan...)
				} else {
					applyIntegration(t, plan, nil)
				}
			}
			if together {
				plan, err := ComposeAgentChanges(plans)
				applyIntegration(t, plan, err)
			}
			installed := read(t, path)
			for i, agent := range agents {
				a, _ := LookupAgent(agent)
				plan, err := a.PlanMCP(s, "", true)
				applyIntegration(t, plan, err)
				got := read(t, path)
				if i < len(agents)-1 && got != installed {
					t.Fatalf("%s uninstall removed another MCP owner: %s", agent, got)
				}
				if i == len(agents)-1 && strings.Contains(got, `"obituary"`) {
					t.Fatal("last owner left MCP registered")
				}
			}
			if _, err := os.Stat(ManagedAssetReceiptPath(path + ".obituary-owners")); !os.IsNotExist(err) {
				t.Fatal("last owner left receipt", err)
			}
		})
	}
}

func TestSharedMCPLegacyReceiptMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	path := filepath.Join(s.Project, ".mcp.json")
	asset := ManagedAsset{Path: path + ".obituary-owners", ID: "codebuddy-mcp", Owner: "codebuddy", Version: "1", Content: []byte("Obituary shared registration: codebuddy-mcp\n")}
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	writeIntegrationFile(t, path, `{"mcpServers":{"obituary":{"command":"obit","args":["mcp"],"env":{"KEEP":"yes"}}}}`)
	plan, err = PlanCopilotMCP(s, "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
	plan, err = PlanCopilotMCP(s, "", true)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), "KEEP") {
		t.Fatal("legacy owner was replaced")
	}
	plan, err = PlanMCP(s, "claude", "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
	plan, err = PlanCodeBuddyMCP(s, "", true)
	applyIntegration(t, plan, err)
	if !strings.Contains(read(t, path), `"obituary"`) {
		t.Fatal("confirmed Claude owner lost registration")
	}
	plan, err = PlanMCP(s, "claude", "", true)
	applyIntegration(t, plan, err)
	if strings.Contains(read(t, path), `"obituary"`) {
		t.Fatal("last confirmed owner did not unregister")
	}
}

func TestSharedMCPLegacyReceiptProtectsUnrecordedClaude(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, owner := range []string{"qoder", "codebuddy", "minimax-code"} {
		t.Run(owner, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, ".mcp.json")
			installLegacyMCP(t, s, path, owner)
			a, _ := LookupAgent(owner)
			plan, err := a.PlanMCP(s, "", true)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(plan)
			if !strings.Contains(read(t, path), `"obituary"`) {
				t.Fatalf("legacy Claude registration removed: %v", err)
			}
			if err == nil || !strings.Contains(err.Error(), "legacy") {
				t.Fatalf("legacy owner needs an explicit verification warning: %v", err)
			}
			receipt := read(t, ManagedAssetReceiptPath(path+".obituary-owners"))
			if !strings.Contains(receipt, "legacy-unverified") || !strings.Contains(receipt, `"schema":2`) {
				t.Fatalf("legacy protection not persisted: %s", receipt)
			}
			plan, err = PlanMCP(s, "claude", "C:/tools/obit.exe", false)
			applyIntegration(t, plan, err)
			if strings.Contains(read(t, ManagedAssetReceiptPath(path+".obituary-owners")), "legacy-unverified") {
				t.Fatal("explicit Claude confirmation did not resolve legacy ownership")
			}
			plan, err = PlanMCP(s, "claude", "", true)
			applyIntegration(t, plan, err)
			if strings.Contains(read(t, path), `"obituary"`) {
				t.Fatal("confirmed last owner left registration")
			}
		})
	}
}

func installLegacyMCP(t *testing.T, s Scope, path, owner string) {
	t.Helper()
	asset := ManagedAsset{Path: path + ".obituary-owners", ID: "pi-omp-mcp", Owner: owner, Version: "1", Content: []byte("Obituary Pi/OMP MCP registration\n")}
	if owner == "codebuddy" {
		asset.ID, asset.Content = "codebuddy-mcp", []byte("Obituary shared registration: codebuddy-mcp\n")
	}
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	writeIntegrationFile(t, path, `{"mcpServers":{"obituary":{"command":"C:/tools/obit.exe","args":["mcp"]}}}`)
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		t.Fatal(err)
	}
	ch, err = planMCPJSON(ch, "C:/tools/obit.exe", false)
	applyIntegration(t, []Change{ch}, err)
}

func TestSharedMCPUninstallRechecksConcurrentOwners(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	plan, err := PlanMCP(s, "claude", "C:/tools/obit.exe", false)
	applyIntegration(t, plan, err)
	plan, err = PlanMCP(s, "claude", "", true)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Project, ".mcp.json")
	beforeIntegrationWrite = func(writing string) error {
		if writing != path {
			return nil
		}
		beforeIntegrationWrite = nil
		other, err := PlanCopilotMCP(s, "C:/tools/obit.exe", false)
		if err != nil {
			return err
		}
		return Apply(other)
	}
	t.Cleanup(func() { beforeIntegrationWrite = nil })
	err = Apply(plan)
	if !strings.Contains(read(t, path), `"obituary"`) {
		t.Fatalf("concurrent owner registration was deleted: %v", err)
	}
	if err == nil {
		t.Fatal("stale owner receipt was accepted")
	}
	plan, err = PlanMCP(s, "claude", "", true)
	applyIntegration(t, plan, err)
	plan, err = PlanCopilotMCP(s, "", true)
	applyIntegration(t, plan, err)
}

func TestSharedMCPRequiresRecordedOwner(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "codebuddy", "qoder", "minimax-code", "copilot"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			s := Scope{Project: t.TempDir()}
			path := filepath.Join(s.Project, ".mcp.json")
			original := `{"mcpServers":{"obituary":{"command":"obit","args":["mcp"]}}}`
			writeIntegrationFile(t, path, original)
			a, _ := LookupAgent(agent)
			if _, err := a.PlanMCP(s, "C:/tools/obit.exe", false); err == nil {
				t.Fatal("unrecorded shared registration adopted")
			}
			plan, err := a.PlanMCP(s, "", true)
			if err != nil {
				t.Fatal(err)
			}
			if Apply(plan) == nil {
				t.Fatal("missing owner conflict not reported")
			}
			if read(t, path) != original {
				t.Fatal("unrecorded shared registration removed")
			}
		})
	}
}

func TestClaudeSharedMCPOwnershipOrders(t *testing.T) {
	t.Parallel()
	for _, domestic := range []string{"qoder", "minimax-code", "codebuddy"} {
		for _, first := range []string{domestic, "claude"} {
			for _, removeFirst := range []string{domestic, "claude"} {
				t.Run(domestic+"/install-"+first+"/uninstall-"+removeFirst, func(t *testing.T) {
					t.Parallel()
					s := Scope{Project: t.TempDir()}
					second := domestic
					if first == domestic {
						second = "claude"
					}
					for _, agent := range []string{first, second} {
						a, _ := LookupAgent(agent)
						plan, err := a.PlanMCP(s, "C:/tools/obit.exe", false)
						applyIntegration(t, plan, err)
					}
					a, _ := LookupAgent(removeFirst)
					plan, err := a.PlanMCP(s, "", true)
					applyIntegration(t, plan, err)
					path := filepath.Join(s.Project, ".mcp.json")
					if !strings.Contains(read(t, path), `"obituary"`) {
						t.Fatal("Claude shared owner registration lost")
					}
					last := domestic
					if removeFirst == domestic {
						last = "claude"
					}
					a, _ = LookupAgent(last)
					plan, err = a.PlanMCP(s, "", true)
					applyIntegration(t, plan, err)
					if strings.Contains(read(t, path), `"obituary"`) {
						t.Fatal("last shared owner retained registration")
					}
				})
			}
		}
	}
}
