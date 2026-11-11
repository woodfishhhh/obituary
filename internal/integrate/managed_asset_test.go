package integrate

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/managed-plugin.ts
var managedPluginFixture []byte

func pluginAsset(s Scope) ManagedAsset {
	return ManagedAsset{Path: filepath.Join(s.Project, "plugins", "obituary.ts"), ID: "pi-extension", Owner: "pi", Version: "1", Content: managedPluginFixture}
}

func TestManagedAssetLifecycleAndSharedOwners(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	asset := pluginAsset(s)
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	if read(t, asset.Path) != string(managedPluginFixture) {
		t.Fatal("embedded asset changed")
	}
	plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, false)
	if err != nil || Pending(plan) {
		t.Fatalf("not idempotent: %+v %v", plan, err)
	}
	asset.Owner = "omp"
	plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	asset.Content = []byte("different")
	if _, err := PlanManagedAssets(s, []ManagedAsset{asset}, false); err == nil {
		t.Fatal("shared payload overwritten")
	}
	plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, true)
	applyIntegration(t, plan, err)
	if read(t, asset.Path) != string(managedPluginFixture) {
		t.Fatal("shared asset removed")
	}
	asset.Owner, asset.Version = "pi", "2"
	plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	if read(t, asset.Path) != "different" || !strings.Contains(read(t, testAssetReceiptPath(t, asset.Path)), `"version":"2"`) {
		t.Fatal("upgrade failed")
	}
	user := filepath.Join(filepath.Dir(asset.Path), "user.ts")
	writeIntegrationFile(t, user, "user")
	plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, true)
	applyIntegration(t, plan, err)
	if _, err := os.Stat(asset.Path); !os.IsNotExist(err) {
		t.Fatal("asset remains", err)
	}
	if read(t, user) != "user" {
		t.Fatal("directory content removed")
	}
}

func TestManagedAssetEmptyFile(t *testing.T) {
	t.Parallel()
	s := Scope{Project: t.TempDir()}
	asset := pluginAsset(s)
	asset.Content = nil
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	applyIntegration(t, plan, err)
	if read(t, asset.Path) != "" {
		t.Fatal("empty asset content changed")
	}
	plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, false)
	if err != nil || Pending(plan) {
		t.Fatalf("empty asset is not idempotent: %+v %v", plan, err)
	}
}

func TestManagedAssetInterruptedWriteRecovery(t *testing.T) {
	t.Parallel()
	for _, upgrade := range []bool{false, true} {
		for _, stopAt := range []string{"pending", "file", "committed"} {
			t.Run(stopAt+map[bool]string{true: "/upgrade", false: "/new"}[upgrade], func(t *testing.T) {
				t.Parallel()
				s := Scope{Project: t.TempDir()}
				asset := pluginAsset(s)
				if upgrade {
					old := asset
					old.Content = []byte("old")
					plan, err := PlanManagedAssets(s, []ManagedAsset{old}, false)
					applyIntegration(t, plan, err)
				}
				plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
				if err != nil {
					t.Fatal(err)
				}
				writes := 0
				stop := errors.New("interrupted")
				afterIntegrationWrite := func(path string) error {
					writes++
					if writes == map[string]int{"pending": 1, "file": 2, "committed": 3}[stopAt] {
						return stop
					}
					return nil
				}

				if err := apply(plan, afterIntegrationWrite); !errors.Is(err, stop) {
					t.Fatalf("interruption missing: %v", err)
				}
				plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, false)
				applyIntegration(t, plan, err)
				if read(t, asset.Path) != string(asset.Content) || strings.Contains(read(t, testAssetReceiptPath(t, asset.Path)), "previous") {
					t.Fatal("recovery failed")
				}
			})
		}
	}
}

func TestManagedAssetPreservesEditsReceiptsAndConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, edit := range []string{"file", "receipt", "orphan receipt"} {
		t.Run(edit, func(t *testing.T) {
			s := Scope{Project: t.TempDir()}
			asset := pluginAsset(s)
			plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
			applyIntegration(t, plan, err)
			target := asset.Path
			if edit != "file" {
				target = testAssetReceiptPath(t, asset.Path)
			}
			writeIntegrationFile(t, target, "user edited")
			if edit == "orphan receipt" {
				if err := os.Remove(asset.Path); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := PlanManagedAssets(s, []ManagedAsset{asset}, false); err == nil {
				t.Fatal("user edit overwritten")
			}
			plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, true)
			if err != nil || PreservedError(plan) == nil || Apply(plan) == nil || read(t, target) != "user edited" {
				t.Fatalf("conflict not reported/preserved: %+v %v", plan, err)
			}
		})
	}
	s := Scope{Project: t.TempDir()}
	asset := pluginAsset(s)
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationFile(t, asset.Path, "saved after plan")
	if err := Apply(plan); err == nil || read(t, asset.Path) != "saved after plan" {
		t.Fatal("concurrent edit overwritten")
	}
	if _, err := os.Stat(testAssetReceiptPath(t, asset.Path)); !os.IsNotExist(err) {
		t.Fatal("receipt written before preflight")
	}
	outside := t.TempDir()
	s = Scope{Project: t.TempDir()}
	makeIntegrationJunction(t, filepath.Join(s.Project, "plugins"), outside)
	if _, err := PlanManagedAssets(s, []ManagedAsset{pluginAsset(s)}, false); err == nil {
		t.Fatal("project escape accepted")
	}
}

func TestManagedAssetOrphanRecoveryAndUnownedCopies(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	asset := pluginAsset(s)
	writeIntegrationFile(t, asset.Path, string(asset.Content))
	if _, err := PlanManagedAssets(s, []ManagedAsset{asset}, false); err == nil {
		t.Fatal("claimed identical unowned copy")
	}
	plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, true)
	if err != nil || PreservedError(plan) == nil || Apply(plan) == nil || read(t, asset.Path) != string(asset.Content) {
		t.Fatal("unowned copy not preserved")
	}
	for _, pending := range []bool{false, true} {
		s = Scope{Project: t.TempDir()}
		asset = pluginAsset(s)
		state := assetReceipt{Schema: 1, ID: asset.ID, File: filepath.Base(asset.Path), Version: "1", Owners: []string{asset.Owner}, SHA256: assetHash(asset.Content)}
		if pending {
			state.Previous = "absent"
		}
		raw, _ := marshal(state)
		writeIntegrationFile(t, testAssetReceiptPath(t, asset.Path), string(raw)+"\n")
		plan, err := PlanManagedAssets(s, []ManagedAsset{asset}, false)
		applyIntegration(t, plan, err)
		if read(t, asset.Path) != string(asset.Content) {
			t.Fatal("orphaned receipt could not recover")
		}
		if err := os.Remove(asset.Path); err != nil {
			t.Fatal(err)
		}
		plan, err = PlanManagedAssets(s, []ManagedAsset{asset}, true)
		applyIntegration(t, plan, err)
		if _, err := os.Stat(testAssetReceiptPath(t, asset.Path)); !os.IsNotExist(err) {
			t.Fatal("orphaned owned receipt retained", err)
		}
	}
}

func TestManagedInstallationOrderAndUnregistrationFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	s := Scope{Project: t.TempDir()}
	asset := pluginAsset(s)
	manifest := ManagedAsset{Path: filepath.Join(s.Project, "plugins", "plugin.json"), ID: "plugin-manifest", Owner: "pi", Version: "1", Content: []byte(`{"name":"obituary"}`), Activation: true}
	assets, err := PlanManagedAssets(s, []ManagedAsset{manifest, asset}, false)
	if err != nil {
		t.Fatal(err)
	}
	registrationPath := filepath.Join(s.Project, "registration.json")
	registration, err := PlanConfigEdits(s, registrationPath, ConfigJSON, []ConfigEdit{{Path: []string{"plugin"}, Value: []byte(`"obituary"`)}})
	if err != nil {
		t.Fatal(err)
	}
	var writes []string
	afterIntegrationWrite := func(path string) error { writes = append(writes, path); return nil }

	plan, err := OrderManagedInstallation(assets, []Change{registration}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(plan, afterIntegrationWrite); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 7 || writes[1] != asset.Path || writes[4] != manifest.Path || writes[6] != registrationPath {
		t.Fatal("unsafe installation order", writes)
	}
	assets, err = PlanManagedAssets(s, []ManagedAsset{manifest, asset}, true)
	if err != nil {
		t.Fatal(err)
	}
	registration, err = PlanConfigEdits(s, registrationPath, ConfigJSON, []ConfigEdit{{Path: []string{"plugin"}, Remove: true}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err = OrderManagedInstallation(assets, []Change{registration}, true)
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("stop after unregister")
	afterIntegrationWrite = func(path string) error {
		if path == registrationPath {
			if read(t, asset.Path) != string(asset.Content) || read(t, manifest.Path) != string(manifest.Content) {
				t.Fatal("assets removed before unregistration")
			}
			return stop
		}
		return nil
	}
	if err := apply(plan, afterIntegrationWrite); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if _, err := OrderManagedInstallation(assets, []Change{{Action: "preserve", Detail: "failed unregister"}}, true); err == nil {
		t.Fatal("accepted incomplete unregistration")
	}
}
