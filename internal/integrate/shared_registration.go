package integrate

import "slices"

// Shared registrations use the existing managed-asset receipt protocol. The
// small sidecar owns a member/block, never the user's entire configuration.
type sharedRegistrationPlan struct {
	asset        managedAssetPlan
	registration []Change
}

func planSharedRegistration(s Scope, asset ManagedAsset, registration []Change, requested []string, uninstall bool) ([]Change, error) {
	file, err := readIntegrationChange(s, asset.Path)
	if err != nil {
		return nil, err
	}
	receipt, err := readIntegrationChange(s, ManagedAssetReceiptPath(asset.Path))
	if err != nil {
		return nil, err
	}
	return planSharedOwners(sharedRegistrationPlan{
		asset:        managedAssetPlan{asset: asset, owners: requested, uninstall: uninstall, file: file, receipt: receipt},
		registration: registration,
	})
}

func planSharedOwners(intent sharedRegistrationPlan) (changes []Change, err error) {
	p := intent.asset
	defer func() {
		for i := range changes {
			changes[i].shared = &intent
			changes[i].asset = nil
		}
	}()
	// Do not relinquish evidence when a user-edited member prevents removal.
	if p.uninstall && PreservedError(intent.registration) != nil {
		return append([]Change{p.file, p.receipt}, intent.registration...), nil
	}
	owners, err := planAssetOwners(p.file, p.receipt, p.asset, p.owners, p.uninstall)
	if err != nil {
		return nil, err
	}
	registration := slices.Clone(intent.registration)
	state, valid := decodeAssetReceipt(p.receipt.orig, p.asset)
	state = migrateSharedMCPReceipt(state, p.asset, p.owners, p.uninstall)
	member := slices.ContainsFunc(p.owners, func(owner string) bool { return slices.Contains(state.Owners, owner) })
	remaining := slices.ContainsFunc(state.Owners, func(owner string) bool { return !slices.Contains(p.owners, owner) })
	if p.uninstall && (!valid || !member || remaining || PreservedError(owners) != nil) {
		for i, ch := range registration {
			changed := ch.Action != "unchanged"
			ch.Action, ch.data, ch.delete = "unchanged", nil, false
			if changed && (!valid || PreservedError(owners) != nil) {
				ch.Action, ch.Detail = "preserve", "shared ownership is unverified; registration preserved"
			}
			if changed && onlyUnverifiedMCPOwners(state, p.asset, p.owners) {
				ch.Action, ch.Detail = "preserve", "legacy shared MCP owners need manual verification; registration retained; rerun obit init --agent claude --mcp to confirm Claude ownership"
			}
			registration[i] = ch
		}
	}
	// Unlike a plugin activation, a retained shared member is an independent
	// conflict receipt: it must not prevent unrelated hooks from uninstalling.
	for i := range registration {
		registration[i].phase = 20
		if p.uninstall {
			registration[i].phase = -40
		}
	}
	return append(owners, registration...), nil
}
