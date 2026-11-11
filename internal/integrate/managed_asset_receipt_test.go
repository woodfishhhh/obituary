package integrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func relocatedFixture(t *testing.T) (Scope, ManagedAsset, string) {
	t.Helper()
	s := Scope{Global: true, Project: t.TempDir()}
	asset := pluginAsset(s)
	return s, asset, filepath.Join(s.Project, "receipts", "asset.json")
}

func TestManagedAssetReceiptInterruptedMigration(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		stops := 1
		if upgrade {
			stops = 3
		}
		for stopAt := 1; stopAt <= stops; stopAt++ {
			t.Run(fmt.Sprintf("upgrade=%t/write=%d", upgrade, stopAt), func(t *testing.T) {
				s, asset, receipt := relocatedFixture(t)
				plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
				applyIntegration(t, plan, err)
				if upgrade {
					asset.Content = []byte("updated content")
				}
				plan, err = PlanManagedAssetReceipt(s, asset, receipt, false)
				if err != nil {
					t.Fatal(err)
				}
				writes, stop := 0, errors.New("interrupted")
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
				plan, err = PlanManagedAssetReceipt(s, asset, receipt, false)
				applyIntegration(t, plan, err)
				if read(t, asset.Path) != string(asset.Content) || strings.Contains(read(t, receipt), "previous") {
					t.Fatal("migration recovery failed")
				}
				if _, err := os.Stat(ManagedAssetReceiptPath(asset.Path)); !os.IsNotExist(err) {
					t.Fatal("legacy receipt remains", err)
				}
			})
		}
	}
}

func TestManagedAssetReceiptHoldsLockThroughMigrationAndRemoval(t *testing.T) {
	s, asset, receipt := relocatedFixture(t)
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	for _, uninstall := range []bool{false, true} {
		plan, err = PlanManagedAssetReceipt(s, asset, receipt, uninstall)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		beforeIntegrationWrite = func(string) error {
			calls++
			if calls == 1 {
				return nil
			}
			release, ok, err := ledger.TryLockExclusive(receipt + ".lock")
			if ok {
				release()
				t.Fatal("receipt transaction lock released before completion")
			}
			return err
		}
		t.Cleanup(func() { beforeIntegrationWrite = nil })
		if err := Apply(plan); err != nil {
			t.Fatal(err)
		}
		beforeIntegrationWrite = nil
		if calls < 3 {
			t.Fatal("did not exercise receipt transaction", calls)
		}
	}
}

func TestManagedAssetReceiptRechecksLegacyUnderLock(t *testing.T) {
	s, asset, receipt := relocatedFixture(t)
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	plan, err = PlanManagedAssetReceipt(s, asset, receipt, false)
	if err != nil {
		t.Fatal(err)
	}
	legacy := ManagedAssetReceiptPath(asset.Path)
	beforeIntegrationWrite = func(string) error {
		beforeIntegrationWrite = nil
		return os.WriteFile(legacy, []byte("edited during lock acquisition"), 0600)
	}
	t.Cleanup(func() { beforeIntegrationWrite = nil })
	if err := Apply(plan); err == nil {
		t.Fatal("stale migration accepted")
	}
	if read(t, legacy) != "edited during lock acquisition" {
		t.Fatal("legacy receipt edit lost")
	}
	if _, err := os.Stat(receipt); !os.IsNotExist(err) {
		t.Fatal("new receipt written before locked recheck", err)
	}
}

func TestManagedAssetReceiptUninstallPreservesOtherOwner(t *testing.T) {
	s, asset, receipt := relocatedFixture(t)
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	asset.Owner = "another-owner"
	plan, err = PlanManagedAssetReceipt(s, asset, receipt, true)
	applyIntegration(t, plan, err)
	if Pending(plan) || read(t, asset.Path) != string(asset.Content) {
		t.Fatal("removed another owner's asset")
	}
	if _, err := os.Stat(ManagedAssetReceiptPath(asset.Path)); err != nil {
		t.Fatal("removed another owner's receipt", err)
	}
}

func TestManagedAssetReceiptRejectsCorruptNewReceiptWithValidLegacy(t *testing.T) {
	s, asset, receipt := relocatedFixture(t)
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	writeIntegrationFile(t, receipt, "user data")
	if _, err := PlanManagedAssetReceipt(s, asset, receipt, false); err == nil {
		t.Fatal("corrupt new receipt replaced from legacy")
	}
	plan, err = PlanManagedAssetReceipt(s, asset, receipt, true)
	if err != nil || PreservedError(plan) == nil || Apply(plan) == nil {
		t.Fatal("uninstall did not preserve corrupt receipt", err)
	}
	if read(t, receipt) != "user data" || read(t, asset.Path) != string(asset.Content) {
		t.Fatal("unverified files changed")
	}
}
