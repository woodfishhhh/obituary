package integrate

import "slices"

const legacyMCPOwner = "legacy-unverified"

func validAssetSchema(schema int, asset ManagedAsset) bool {
	return schema == 1 || asset.receiptSchema == 2 && schema == 2
}

// Schema 1 predates Claude/Copilot participation. Preserve its unknown owners
// through domestic refreshes; an explicit Claude install confirms that usage.
func migrateSharedMCPReceipt(state assetReceipt, asset ManagedAsset, requested []string, uninstall bool) assetReceipt {
	if asset.receiptSchema != 2 || state.Schema == 0 {
		return state
	}
	state.Owners = slices.Clone(state.Owners)
	if state.Schema == 1 && !slices.Contains(state.Owners, legacyMCPOwner) {
		state.Owners = append(state.Owners, legacyMCPOwner)
	}
	if !uninstall && slices.Contains(requested, "claude") {
		state.Owners = slices.DeleteFunc(state.Owners, func(owner string) bool { return owner == legacyMCPOwner })
	}
	state.Schema = 2
	slices.Sort(state.Owners)
	return state
}

func onlyUnverifiedMCPOwners(state assetReceipt, asset ManagedAsset, requested []string) bool {
	if asset.receiptSchema != 2 || !slices.Contains(state.Owners, legacyMCPOwner) {
		return false
	}
	return !slices.ContainsFunc(state.Owners, func(owner string) bool {
		return owner != legacyMCPOwner && !slices.Contains(requested, owner)
	})
}
