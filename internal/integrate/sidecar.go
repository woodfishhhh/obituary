package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// The receipt keeps its legacy file field while the storage name uses the
// normalized absolute legacy path, shared with shortcut receipt naming.
func sidecarPath(path string) (string, error) {
	return userdirs.ReceiptPath(path, "integration-receipts", filepath.Ext(path))
}

func planProjectAsset(s Scope, asset ManagedAsset, uninstall bool) ([]Change, error) {
	path, err := userdirs.ReceiptPath(asset.Path, "integration-receipts", ".obituary-asset")
	if err != nil {
		return nil, err
	}
	return PlanManagedAssetReceipt(s, asset, path, uninstall)
}

func readAssetReceipt(s Scope, path string) (Change, error) {
	if !s.Global {
		receiptPath, err := userdirs.ReceiptPath(path, "integration-receipts", ".obituary-asset")
		if err != nil {
			return Change{}, err
		}
		receipt, err := readIntegrationChange(dataScope(s), receiptPath)
		if err != nil || receipt.existed {
			return receipt, err
		}
	}
	return readIntegrationChange(s, ManagedAssetReceiptPath(path))
}

func dataScope(s Scope) Scope {
	s.Global, s.Project = true, ""
	return s
}

type sidecarMigration struct {
	file, receipt             Change
	legacyFile, legacyReceipt Change
	legacyLock                Change
}

func readSharedAsset(s Scope, asset ManagedAsset) (managedAssetPlan, error) {
	p := managedAssetPlan{asset: asset}
	path, err := sidecarPath(asset.Path)
	if err != nil {
		return p, err
	}
	m := &sidecarMigration{}
	for _, target := range []struct {
		ch    *Change
		scope Scope
		path  string
	}{
		{&m.file, dataScope(s), path}, {&m.receipt, dataScope(s), ManagedAssetReceiptPath(path)},
		{&m.legacyFile, s, asset.Path}, {&m.legacyReceipt, s, ManagedAssetReceiptPath(asset.Path)},
		{&m.legacyLock, s, asset.Path + ".lock"},
	} {
		*target.ch, err = readIntegrationChange(target.scope, target.path)
		if err != nil {
			return p, err
		}
	}
	p.file, p.receipt, p.migration = m.file, m.receipt, m
	if !m.file.existed && !m.receipt.existed {
		p.file, p.receipt = m.legacyFile, m.legacyReceipt
	}
	return p, nil
}

func readSharedReceipt(s Scope, legacyPath string) (Change, error) {
	path, err := sidecarPath(legacyPath)
	if err != nil {
		return Change{}, err
	}
	receipt, err := readIntegrationChange(dataScope(s), ManagedAssetReceiptPath(path))
	if err != nil || receipt.existed {
		return receipt, err
	}
	return readIntegrationChange(s, ManagedAssetReceiptPath(legacyPath))
}

func relocateRemainingOwners(plan []Change, p managedAssetPlan) ([]Change, error) {
	// Recreate the verified sidecar through pending/file/commit before dropping
	// the old copy; other owners must survive an interrupted partial uninstall.
	state, valid := decodeAssetReceipt(plan[1].data, p.asset)
	if !valid {
		return nil, fmt.Errorf("invalid remaining owners for %s", p.asset.Path)
	}
	asset := p.asset
	asset.Content = p.file.orig
	receipt := p.receipt
	receipt.orig = plan[1].data
	return planAssetOwners(p.migration.file, receipt, asset, state.Owners, false)
}

func relocateSidecarWrites(plan []Change, p managedAssetPlan) ([]Change, error) {
	m := p.migration
	if p.file.Path != m.legacyFile.Path {
		return plan, nil
	}
	var err error
	switch {
	case p.uninstall && plan[1].delete:
		plan[0] = relocateAssetReceipt(plan[0], m.file, true)
	case p.uninstall:
		plan, err = relocateRemainingOwners(plan, p)
	default:
		plan, err = planAssetOwners(m.file, p.receipt, p.asset, p.owners, false)
	}
	if err != nil {
		return nil, err
	}
	plan[1] = relocateAssetReceipt(plan[1], m.receipt, p.uninstall)
	return plan, nil
}

func migrateSidecarPlan(plan []Change, p managedAssetPlan) ([]Change, error) {
	m := p.migration
	if m == nil || PreservedError(plan) != nil {
		return plan, nil
	}
	if p.uninstall && !Pending(plan) && (!m.receipt.existed || !m.legacyReceipt.existed) {
		return plan, nil
	}
	if m.legacyFile.existed || m.legacyReceipt.existed {
		state, valid := decodeAssetReceipt(m.legacyReceipt.orig, p.asset)
		if !valid || m.legacyFile.existed && state.SHA256 != assetHash(m.legacyFile.orig) && state.Previous != assetHash(m.legacyFile.orig) {
			return nil, fmt.Errorf("%s: unverified legacy sidecar; left untouched", m.legacyFile.Path)
		}
	}
	var err error
	plan, err = relocateSidecarWrites(plan, p)
	if err != nil {
		return nil, err
	}
	intent := &relocatedAssetPlan{receipt: m.receipt, checks: []Change{m.file, m.receipt, m.legacyFile, m.legacyReceipt}}
	for _, legacy := range []Change{m.legacyFile, m.legacyReceipt} {
		if legacy.existed {
			legacy.Action, legacy.delete, legacy.phase = "remove", true, 50
			plan = append(plan, legacy)
		}
	}
	if m.legacyReceipt.existed && m.legacyLock.existed && len(m.legacyLock.orig) == 0 {
		lock := m.legacyLock
		lock.Action, lock.delete, lock.phase = "remove", true, 49
		plan = append(plan, lock)
	}
	for i := range plan {
		plan[i].relocated = intent
		if plan[i].next != nil {
			plan[i].next.relocated = intent
		}
	}
	return plan, nil
}
