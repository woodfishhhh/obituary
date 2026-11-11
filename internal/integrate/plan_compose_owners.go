package integrate

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// A composed configuration can contain hooks and shared MCP edits. Keep its
// sidecar and receipt in the same Apply batch so the resource lock spans all
// three writes, even when the original planners used separate extension groups.
func bindSharedTransactionGroups(changes []Change) {
	for _, intent := range sharedApplyIntents(changes) {
		group, implicit := "", true
		for _, ch := range changes {
			if ch.shared == intent {
				group, implicit = ch.Extension, ch.Implicit
				break
			}
		}
		for _, registration := range intent.registration {
			for _, ch := range changes {
				if strings.EqualFold(ch.Path, registration.Path) && ch.Action != "unchanged" {
					if group != ch.Extension {
						group = ""
					}
					implicit = implicit && ch.Implicit
				}
			}
		}
		for i := range changes {
			if changes[i].shared == intent {
				changes[i].Extension, changes[i].Implicit = group, implicit
			}
		}
	}
}

// Merge the owner intentions first, then produce one pending/file/commit
// transaction. Combining serialized receipt JSON would lose pending recovery
// state and cannot decide whether the final owner is leaving.
func composeOwnerChanges(changes []Change) ([]Change, error) {
	groups, order := composeOwnerGroups(changes)
	replacements := map[int][]Change{}
	removed := map[int]bool{}
	for _, key := range order {
		indices := groups[key]
		plan, combined, err := composeOwnerGroup(changes, indices, key)
		if err != nil {
			return nil, err
		}
		if !combined {
			continue
		}
		replacements[indices[0]] = plan
		for _, i := range indices {
			removed[i] = true
		}
	}
	var out []Change
	for i, ch := range changes {
		if plan, ok := replacements[i]; ok {
			out = append(out, plan...)
		}
		if !removed[i] {
			out = append(out, ch)
		}
	}
	return out, nil
}

func composeOwnerGroups(changes []Change) (map[string][]int, []string) {
	groups := map[string][]int{}
	var order []string
	for i, ch := range changes {
		var path string
		if ch.shared != nil {
			path = ch.shared.asset.asset.Path
		} else if ch.asset != nil {
			path = ch.asset.asset.Path
		}
		if path == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(path))
		if len(groups[key]) == 0 {
			order = append(order, key)
		}
		groups[key] = append(groups[key], i)
	}
	return groups, order
}

func collectOwnerPlans(changes []Change, indices []int, key string) ([]managedAssetPlan, []Change, error) {
	var plans []managedAssetPlan
	var registration []Change
	seenShared := map[*sharedRegistrationPlan]bool{}
	seenAsset := map[*managedAssetPlan]bool{}
	shared := changes[indices[0]].shared != nil
	for _, i := range indices {
		ch := changes[i]
		if (ch.shared != nil) != shared {
			return nil, nil, fmt.Errorf("%s: mixed shared ownership plans", key)
		}
		if shared {
			if !seenShared[ch.shared] {
				plans = append(plans, ch.shared.asset)
				registration = append(registration, ch.shared.registration...)
				seenShared[ch.shared] = true
			}
		} else if !seenAsset[ch.asset] {
			plans = append(plans, *ch.asset)
			seenAsset[ch.asset] = true
		}
	}
	return plans, registration, nil
}

func mergeOwnerPlans(plans []managedAssetPlan, key string) (managedAssetPlan, error) {
	first := plans[0]
	requested := slices.Clone(first.owners)
	for _, p := range plans[1:] {
		if p.uninstall != first.uninstall || p.asset.ID != first.asset.ID || p.asset.Version != first.asset.Version || p.asset.receiptSchema != first.asset.receiptSchema || p.asset.Activation != first.asset.Activation || !bytes.Equal(p.asset.Content, first.asset.Content) {
			return first, fmt.Errorf("%s: conflicting shared asset intentions", key)
		}
		if err := nativeSpecialOriginal(p.file, first.file); err != nil {
			return first, err
		}
		if err := nativeSpecialOriginal(p.receipt, first.receipt); err != nil {
			return first, err
		}
		if p.file.project != first.file.project || p.receipt.project != first.receipt.project {
			return first, fmt.Errorf("%s: differing owner boundaries", key)
		}
		requested = append(requested, p.owners...)
	}
	slices.Sort(requested)
	first.owners = slices.Compact(requested)
	return first, nil
}

func composeOwnerGroup(changes []Change, indices []int, key string) ([]Change, bool, error) {
	plans, registration, err := collectOwnerPlans(changes, indices, key)
	if err != nil {
		return nil, false, err
	}
	if len(plans) == 1 {
		return nil, false, nil
	}
	first, err := mergeOwnerPlans(plans, key)
	if err != nil {
		return nil, false, err
	}
	var plan []Change
	if changes[indices[0]].shared != nil {
		registration, err = ComposeAgentChanges(registration)
		if err == nil {
			plan, err = planSharedOwners(sharedRegistrationPlan{asset: first, registration: registration})
		}
	} else {
		plan, err = planAssetOwners(first.file, first.receipt, first.asset, first.owners, first.uninstall)
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", key, err)
	}
	for j := range plan {
		plan[j], err = composedOwnerMetadata(plan[j], changes, indices, key)
		if err != nil {
			return nil, false, err
		}
	}
	return plan, true, nil
}

func composedOwnerMetadata(ch Change, changes []Change, indices []int, key string) (Change, error) {
	var matched bool
	for _, i := range indices {
		original := changes[i]
		if !strings.EqualFold(original.Path, ch.Path) {
			continue
		}
		if !matched {
			ch.Extension, ch.Implicit, ch.project, ch.phase = original.Extension, original.Implicit, original.project, original.phase
			matched = true
			continue
		}
		if ch.Extension != original.Extension {
			ch.Extension = ""
		}
		ch.Implicit = ch.Implicit && original.Implicit
		if ch.project != original.project {
			return ch, fmt.Errorf("%s: differing composed owner boundaries", key)
		}
	}
	if ch.next != nil {
		ch.next.project = ch.project
	}
	return ch, nil
}
