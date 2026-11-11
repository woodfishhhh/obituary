package integrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
)

// ManagedAsset is one embedded plugin file, optionally rendered with the fixed
// executable path. ID identifies the shared asset family; Owner is the agent.
// Activation marks a loader manifest that must be written last/removed first.
type ManagedAsset struct {
	Path, ID, Owner, Version string
	Content                  []byte
	Activation               bool
	receiptSchema            int // zero keeps the original managed-asset protocol
}

type assetReceipt struct {
	Schema   int      `json:"schema"`
	ID       string   `json:"id"`
	File     string   `json:"file"`
	Version  string   `json:"version"`
	Owners   []string `json:"owners"`
	SHA256   string   `json:"sha256"`
	Previous string   `json:"previous,omitempty"` // pending: old hash or "absent"
}

func ManagedAssetReceiptPath(path string) string { return path + ".obituary-asset" }

// PlanManagedAssets writes pending receipts, files, then committed receipts.
// It never removes directories. Edited assets and malformed receipts are
// conflicts on install and explicit preserved changes on uninstall.
func PlanManagedAssets(s Scope, assets []ManagedAsset, uninstall bool) ([]Change, error) {
	var changes []Change
	seen := map[string]bool{}
	for _, asset := range assets {
		if asset.ID == "" || asset.Owner == "" || asset.Version == "" || !filepath.IsAbs(asset.Path) {
			return nil, fmt.Errorf("managed asset requires absolute path, ID, owner and version")
		}
		for _, path := range []string{asset.Path, ManagedAssetReceiptPath(asset.Path)} {
			key := strings.ToLower(filepath.Clean(path))
			if seen[key] {
				return nil, fmt.Errorf("duplicate managed asset destination %s", path)
			}
			seen[key] = true
		}
		plan, err := planManagedAsset(s, asset, uninstall)
		if err != nil {
			return nil, err
		}
		changes = append(changes, plan...)
	}
	return changes, nil
}

// OrderManagedInstallation orders registration after durable asset commits,
// and unregistration before asset removal. Callers pass both plans to Apply
// together so its shared preflight detects concurrent edits before any write.
func OrderManagedInstallation(assets, registration []Change, uninstall bool) ([]Change, error) {
	if uninstall && PreservedError(registration) != nil {
		return nil, fmt.Errorf("plugin registration could not be removed: %w", PreservedError(registration))
	}
	out := slices.Clone(assets)
	for _, change := range registration {
		if change.next != nil {
			return nil, fmt.Errorf("registration changes cannot contain receipt transactions")
		}
		change.phase = 20
		if uninstall {
			change.phase = -40
		}
		out = append(out, change)
	}
	return out, nil
}

func planManagedAsset(s Scope, asset ManagedAsset, uninstall bool) ([]Change, error) {
	file, err := readIntegrationChange(s, asset.Path)
	if err != nil {
		return nil, err
	}
	receipt, err := readIntegrationChange(s, ManagedAssetReceiptPath(asset.Path))
	if err != nil {
		return nil, err
	}
	return planAssetOwners(file, receipt, asset, []string{asset.Owner}, uninstall)
}

type managedAssetPlan struct {
	asset         ManagedAsset
	owners        []string
	uninstall     bool
	file, receipt Change
}

func planAssetOwners(file, receipt Change, asset ManagedAsset, requested []string, uninstall bool) (changes []Change, err error) {
	intent := &managedAssetPlan{asset: asset, owners: requested, uninstall: uninstall, file: file, receipt: receipt}
	defer func() {
		for i := range changes {
			changes[i].asset = intent
		}
	}()
	state, valid := decodeAssetReceipt(receipt.orig, asset)
	owned := valid && (!file.existed || state.SHA256 == assetHash(file.orig) || state.Previous == assetHash(file.orig))
	if file.existed && !owned || receipt.existed && !owned {
		if uninstall {
			file.Action, file.Detail = "preserve", "unowned or user-edited managed asset preserved"
			receipt.Action, receipt.Detail = "preserve", "unverified asset receipt preserved"
			return []Change{file, receipt}, nil
		}
		return nil, fmt.Errorf("%s is unowned or changed since installation; left untouched", asset.Path)
	}
	state = migrateSharedMCPReceipt(state, asset, requested, uninstall)
	owners := slices.Clone(state.Owners)
	if uninstall {
		if !owned || !slices.ContainsFunc(requested, func(owner string) bool { return slices.Contains(owners, owner) }) {
			return []Change{file, receipt}, nil
		}
		owners = slices.DeleteFunc(owners, func(owner string) bool { return slices.Contains(requested, owner) })
		if len(owners) > 0 {
			state.Owners = owners
			data, _ := marshal(state)
			receipt = setIntegrationData(receipt, append(data, '\n'))
		} else {
			if file.existed {
				file.Action, file.delete = "remove", true
			}
			receipt.Action, receipt.delete = "remove", true
		}
		file.phase, receipt.phase = -20, -10
		if asset.Activation {
			file.phase, receipt.phase = -30, -25
		}
		return []Change{file, receipt}, nil
	}
	for _, owner := range requested {
		if !slices.Contains(owners, owner) {
			owners = append(owners, owner)
		}
	}
	slices.Sort(owners)
	// Portable skills carry identical embedded bytes for every participating host.
	sharedSkillUpgrade := (asset.ID == sharedSkillID || asset.ID == "qoder-skill") && bytes.Equal(asset.Content, skillContent)
	if file.existed && len(owners) > 1 && !bytes.Equal(file.orig, asset.Content) && !sharedSkillUpgrade {
		return nil, fmt.Errorf("%s is shared by %s; a different payload requires coordinated installation", asset.Path, strings.Join(owners, ", "))
	}
	finalState := assetReceipt{Schema: max(1, asset.receiptSchema), ID: asset.ID, File: filepath.Base(asset.Path), Version: asset.Version, Owners: owners, SHA256: assetHash(asset.Content)}
	finalData, _ := marshal(finalState)
	finalData = append(finalData, '\n')
	file = setIntegrationData(file, bytes.Clone(asset.Content))
	if !file.existed {
		file.Action, file.data = "create", bytes.Clone(asset.Content)
	}
	final := setIntegrationData(receipt, finalData)
	preparePhase, filePhase, commitPhase := -30, -20, -10
	if asset.Activation {
		preparePhase, filePhase, commitPhase = 5, 10, 15
	}
	file.phase, final.phase = filePhase, commitPhase
	if file.Action != "unchanged" {
		finalState.Previous = "absent"
		if file.existed {
			finalState.Previous = assetHash(file.orig)
		}
		pending, _ := marshal(finalState)
		pending = append(pending, '\n')
		receipt = setIntegrationData(receipt, pending)
		receipt.phase = preparePhase
		final.orig, final.existed, final.Action, final.data = pending, true, "update", finalData
		receipt.next = &final
	} else {
		receipt = final
	}
	return []Change{file, receipt}, nil
}

func assetHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func decodeAssetReceipt(data []byte, asset ManagedAsset) (assetReceipt, bool) {
	if asset.ID == sharedSkillID && !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		return decodeLegacySharedSkill(data, asset)
	}
	var state assetReceipt
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(&state) != nil || dec.Decode(new(any)) != io.EOF || !validAssetSchema(state.Schema, asset) || state.ID != asset.ID || state.File != filepath.Base(asset.Path) || state.Version == "" || len(state.Owners) == 0 {
		return state, false
	}
	// Duplicate members and noncanonical receipts can disguise user edits.
	canonical, _ := marshal(state)
	if !bytes.Equal(bytes.TrimSpace(data), canonical) {
		return state, false
	}
	seen := map[string]bool{}
	for _, owner := range state.Owners {
		if owner == "" || seen[owner] {
			return state, false
		}
		seen[owner] = true
	}
	validHash := func(hash string) bool {
		raw, err := hex.DecodeString(hash)
		return err == nil && len(raw) == sha256.Size && hash == strings.ToLower(hash)
	}
	if !validHash(state.SHA256) || state.Previous != "" && state.Previous != "absent" && !validHash(state.Previous) {
		return state, false
	}
	return state, true
}
