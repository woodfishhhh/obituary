package integrate

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

type relocatedAssetPlan struct {
	receipt Change
	checks  []Change
}

// PlanManagedAssetReceipt keeps generated assets' receipts outside visible
// directories. Both receipt locations participate in preflight and locking.
// A verified legacy receipt is removed only after the new receipt commits.
func PlanManagedAssetReceipt(s Scope, asset ManagedAsset, receiptPath string, uninstall bool) ([]Change, error) {
	legacyPath := ManagedAssetReceiptPath(asset.Path)
	if asset.ID == "" || asset.Owner == "" || asset.Version == "" || !filepath.IsAbs(asset.Path) || !filepath.IsAbs(receiptPath) {
		return nil, fmt.Errorf("managed asset requires absolute paths, ID, owner and version")
	}
	if strings.EqualFold(filepath.Clean(receiptPath), filepath.Clean(asset.Path)) || strings.EqualFold(filepath.Clean(receiptPath), filepath.Clean(legacyPath)) {
		return nil, fmt.Errorf("relocated receipt requires a separate destination")
	}
	file, err := readIntegrationChange(s, asset.Path)
	if err != nil {
		return nil, err
	}
	receipt, err := readIntegrationChange(dataScope(s), receiptPath)
	if err != nil {
		return nil, err
	}
	legacy, err := readIntegrationChange(s, legacyPath)
	if err != nil {
		return nil, err
	}
	return planRelocatedAsset(asset, file, receipt, legacy, uninstall)
}

func planRelocatedAsset(asset ManagedAsset, file, receipt, legacy Change, uninstall bool) ([]Change, error) {
	return planRelocatedOwners(asset, file, receipt, legacy, []string{asset.Owner}, uninstall)
}

func planRelocatedOwners(asset ManagedAsset, file, receipt, legacy Change, owners []string, uninstall bool) ([]Change, error) {
	source := receipt
	if !receipt.existed && legacy.existed {
		source = legacy
	}
	plan, err := planAssetOwners(file, source, asset, owners, uninstall)
	if err != nil {
		return nil, err
	}
	if PreservedError(plan) != nil || uninstall && !Pending(plan) {
		return plan, nil
	}
	if legacy.existed {
		if _, valid := decodeAssetReceipt(legacy.orig, asset); !valid {
			return nil, fmt.Errorf("%s: unverified legacy receipt; left untouched", legacy.Path)
		}
	}
	if source.Path == legacy.Path {
		plan[1] = relocateAssetReceipt(plan[1], receipt, uninstall)
	}
	legacy.phase = 0 // after the final receipt, including interrupted-write recovery
	if legacy.existed {
		legacy.Action, legacy.delete = "remove", true
	}
	intent := &relocatedAssetPlan{receipt: receipt, checks: []Change{file, receipt, legacy}}
	if legacy.existed {
		plan = append(plan, legacy)
	}
	ownerIntent := &managedAssetPlan{asset: asset, owners: owners, uninstall: uninstall, file: file, receipt: source, receiptMove: intent}
	for i := range plan {
		// Ownership and content were verified; generated files need no backup.
		plan[i].asset, plan[i].relocated = ownerIntent, intent
		if plan[i].next != nil {
			plan[i].next.relocated = intent
		}
	}
	return plan, nil
}

func relocateAssetReceipt(planned, destination Change, uninstall bool) Change {
	if uninstall && planned.delete {
		return destination
	}
	data := planned.data
	if planned.Action == "unchanged" {
		data = planned.orig
	}
	result := setIntegrationData(destination, data)
	result.phase, result.next = planned.phase, planned.next
	if result.next != nil {
		result.next.Path = destination.Path
		result.next.location = destination.location
		result.next.project, result.next.backup = destination.project, destination.backup
		result.next.identity = nil // the pending write supplies its new identity
	}
	return result
}

func relocatedApplyIntents(changes []Change) []*relocatedAssetPlan {
	var intents []*relocatedAssetPlan
	seen := map[string]bool{}
	for _, ch := range changes {
		if ch.relocated != nil && ch.shared == nil && !seen[ch.relocated.receipt.Path] {
			intents = append(intents, ch.relocated)
			seen[ch.relocated.receipt.Path] = true
		}
	}
	slices.SortFunc(intents, func(a, b *relocatedAssetPlan) int {
		return strings.Compare(a.receipt.Path, b.receipt.Path)
	})
	return intents
}

func (l *sharedApplyLocks) acquireRelocated(changes []Change) error {
	intents := relocatedApplyIntents(changes)
	for _, intent := range intents {
		release, err := lockSharedResource(intent.receipt)
		if err != nil {
			return err
		}
		l.releases = append(l.releases, release)
	}
	for _, intent := range intents {
		for _, ch := range intent.checks {
			if err := checkOriginal(ch); err != nil {
				return err
			}
		}
	}
	return nil
}
