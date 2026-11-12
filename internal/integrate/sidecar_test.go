package integrate

import "github.com/woodfishhhh/obituary/internal/testenv"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func legacySidecar(t *testing.T, s Scope, asset ManagedAsset) {
	t.Helper()
	file, err := readIntegrationChange(s, asset.Path)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := readIntegrationChange(s, ManagedAssetReceiptPath(asset.Path))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planAssetOwners(file, receipt, asset, []string{asset.Owner}, false)
	for i := range plan {
		plan[i].generated = true
		if plan[i].next != nil {
			plan[i].next.generated = true
		}
	}
	applyIntegration(t, plan, err)
}

func TestProjectPlansKeepMetadataInData(t *testing.T) {
	root := agentGoldenEnvironment(t)
	s := Scope{Project: filepath.Join(root, "project")}
	for _, agent := range Agents() {
		plans, _ := agent.Plan(s, ownershipCommands(agent.Name), false)
		if agent.PlanSkill != nil {
			extra, _ := agent.PlanSkill(s, false)
			plans = append(plans, extra...)
		}
		if agent.PlanMCP != nil {
			extra, _ := agent.PlanMCP(s, testenv.NativeFixture("C:/tools/obit.exe"), false)
			plans = append(plans, extra...)
		}
		for _, ch := range plans {
			if ch.Action == "unchanged" || ch.Action == "preserve" {
				continue
			}
			name := filepath.Base(ch.Path)
			if name == ".obituary-skill" {
				continue
			}
			if strings.Contains(name, ".obituary-") || strings.HasSuffix(name, ".owners") {
				rel, err := filepath.Rel(s.Project, ch.Path)
				if err == nil && filepath.IsLocal(rel) {
					t.Errorf("%s plans project metadata: %s", agent.Name, ch.Path)
				}
			}
		}
	}
}

func TestSidecarMigrationRecoversInterruptedWrites(t *testing.T) {
	for stopAt := 1; stopAt <= 3; stopAt++ {
		t.Run(fmt.Sprint(stopAt), func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			asset := ManagedAsset{Path: filepath.Join(s.Project, "CLAUDE.md.obituary-reference"), ID: "claude-reference", Owner: "claude", Version: "1", Content: []byte("Obituary owns only its marked Claude import\n")}
			legacySidecar(t, s, asset)
			writeIntegrationFile(t, asset.Path+".lock", "")
			writeIntegrationFile(t, asset.Path+".bak", "unverified backup")
			asset.Content = []byte("updated sidecar")
			plan, err := planSharedRegistration(s, asset, nil, []string{asset.Owner}, false)
			if err != nil {
				t.Fatal(err)
			}
			stop, writes := errors.New("interrupted"), 0
			err = apply(plan, func(string) error {
				writes++
				if writes == stopAt {
					return stop
				}
				return nil
			})
			if !errors.Is(err, stop) {
				t.Fatal("missing interruption", err)
			}
			plan, err = planSharedRegistration(s, asset, nil, []string{asset.Owner}, false)
			applyIntegration(t, plan, err)
			path := testSidecarPath(t, asset.Path)
			if read(t, path) != string(asset.Content) || strings.Contains(read(t, ManagedAssetReceiptPath(path)), "previous") {
				t.Fatal("migration incomplete")
			}
			for _, old := range []string{asset.Path, ManagedAssetReceiptPath(asset.Path), asset.Path + ".lock"} {
				if _, err := os.Stat(old); !os.IsNotExist(err) {
					t.Fatal("legacy sidecar remains", old, err)
				}
			}
			if read(t, asset.Path+".bak") != "unverified backup" {
				t.Fatal("unverified backup changed")
			}
			plan, err = planSharedRegistration(s, asset, nil, []string{asset.Owner}, true)
			applyIntegration(t, plan, err)
			for _, removed := range []string{path, ManagedAssetReceiptPath(path)} {
				if _, err := os.Stat(removed); !os.IsNotExist(err) {
					t.Fatal("owned sidecar remains", err)
				}
			}
		})
	}
}

func TestSidecarUninstallRemovesLegacyAndNewLocations(t *testing.T) {
	for _, migrate := range []bool{false, true} {
		t.Run(fmt.Sprint(migrate), func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			asset := ManagedAsset{Path: filepath.Join(s.Project, "AGENTS.md.obituary-owners"), ID: "agent-rules", Owner: "codex", Version: "1", Content: []byte("Obituary shared agent instructions\n")}
			legacySidecar(t, s, asset)
			writeIntegrationFile(t, asset.Path+".lock", "")
			if migrate {
				plan, err := planSharedRegistration(s, asset, nil, []string{asset.Owner}, false)
				if err != nil {
					t.Fatal(err)
				}
				stop := errors.New("stop before legacy cleanup")
				beforeIntegrationWrite = func(path string) error {
					if path == asset.Path+".lock" {
						return stop
					}
					return nil
				}
				t.Cleanup(func() { beforeIntegrationWrite = nil })
				if !errors.Is(Apply(plan), stop) {
					t.Fatal("interruption not reached")
				}
				beforeIntegrationWrite = nil
			}
			plan, err := planSharedRegistration(s, asset, nil, []string{asset.Owner}, true)
			applyIntegration(t, plan, err)
			path := testSidecarPath(t, asset.Path)
			for _, removed := range []string{asset.Path, ManagedAssetReceiptPath(asset.Path), asset.Path + ".lock", path, ManagedAssetReceiptPath(path)} {
				if _, err := os.Stat(removed); !os.IsNotExist(err) {
					t.Fatal("owned sidecar remains", removed, err)
				}
			}
		})
	}
}

func TestSidecarMigrationRejectsUnverifiedLegacy(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	asset := ManagedAsset{Path: filepath.Join(s.Project, "AGENTS.md.obituary-owners"), ID: "agent-rules", Owner: "codex", Version: "1", Content: []byte("Obituary shared agent instructions\n")}
	legacySidecar(t, s, asset)
	writeIntegrationFile(t, asset.Path, "user modified sidecar")
	if _, err := planSharedRegistration(s, asset, nil, []string{asset.Owner}, false); err == nil {
		t.Fatal("user sidecar accepted")
	}
	plan, err := planSharedRegistration(s, asset, nil, []string{asset.Owner}, true)
	if err != nil || PreservedError(plan) == nil || Apply(plan) == nil {
		t.Fatal("unverified uninstall not reported", err)
	}
	if read(t, asset.Path) != "user modified sidecar" {
		t.Fatal("user sidecar removed")
	}
	if _, err := os.Stat(testSidecarPath(t, asset.Path)); !os.IsNotExist(err) {
		t.Fatal("unverified sidecar migrated", err)
	}
}

func TestSidecarLegacyUninstallRetainsOtherOwners(t *testing.T) {
	s := Scope{Project: t.TempDir()}
	asset := ManagedAsset{Path: filepath.Join(s.Project, "AGENTS.md.obituary-owners"), ID: "agent-rules", Owner: "codex", Version: "1", Content: []byte("Obituary shared agent instructions\n")}
	legacySidecar(t, s, asset)
	asset.Owner = "hermes"
	legacySidecar(t, s, asset)
	plan, err := planSharedRegistration(s, asset, nil, []string{"hermes"}, true)
	applyIntegration(t, plan, err)
	path := testSidecarPath(t, asset.Path)
	state, valid := decodeAssetReceipt([]byte(read(t, ManagedAssetReceiptPath(path))), asset)
	if !valid || len(state.Owners) != 1 || state.Owners[0] != "codex" || read(t, path) != string(asset.Content) {
		t.Fatal("other owner lost", state)
	}
	plan, err = planSharedRegistration(s, asset, nil, []string{"codex"}, true)
	applyIntegration(t, plan, err)
}

func testSidecarPath(t *testing.T, legacy string) string {
	t.Helper()
	path, err := sidecarPath(legacy)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSidecarMigrationRechecksBothLocations(t *testing.T) {
	for _, changeNew := range []bool{false, true} {
		t.Run(fmt.Sprint(changeNew), func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			asset := ManagedAsset{Path: filepath.Join(s.Project, "AGENTS.md.obituary-owners"), ID: "agent-rules", Owner: "codex", Version: "1", Content: []byte("Obituary shared agent instructions\n")}
			legacySidecar(t, s, asset)
			plan, err := planSharedRegistration(s, asset, nil, []string{asset.Owner}, false)
			if err != nil {
				t.Fatal(err)
			}
			changed := asset.Path
			if changeNew {
				changed = testSidecarPath(t, asset.Path)
			}
			beforeIntegrationWrite = func(string) error {
				beforeIntegrationWrite = nil
				writeIntegrationFile(t, changed, "user edit")
				return nil
			}
			t.Cleanup(func() { beforeIntegrationWrite = nil })
			if Apply(plan) == nil || read(t, changed) != "user edit" {
				t.Fatal("concurrent edit overwritten")
			}
		})
	}
}
