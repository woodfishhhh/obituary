package integrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

const sharedSkillID = "agent-skills-obituary"

// PlanSharedSkill installs one portable skill with an independent owner for each
// caller. The last owner removes it; user edits and unknown receipts are retained.
func PlanSharedSkill(s Scope, owner string, uninstall bool) ([]Change, error) {
	root := filepath.Join(s.Project, ".agents", "skills")
	if s.Global {
		var err error
		root, err = userdirs.AgentSkillsDir()
		if err != nil {
			return nil, err
		}
	}
	return planSkillAsset(s, root, sharedSkillID, owner, skillContent, uninstall)
}

func planSkillAsset(s Scope, root, id, owner string, content []byte, uninstall bool) ([]Change, error) {
	if owner == "" || id == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("skill requires an absolute directory and a nonempty owner and ID")
	}
	if !s.Global && !filepath.IsAbs(s.Project) {
		return nil, fmt.Errorf("%s skill requires an absolute project directory", owner)
	}
	path := filepath.Join(root, "obituary", "SKILL.md")
	asset := ManagedAsset{Path: path, ID: id, Owner: owner, Version: "1", Content: content}
	bound := nativeSpecialScope(s, root)
	file, err := readIntegrationChange(bound, path)
	if err != nil {
		return nil, err
	}
	receiptPath := ManagedAssetReceiptPath(path)
	if id == sharedSkillID {
		receiptPath = filepath.Join(filepath.Dir(path), ".obituary-skill")
	}
	receipt, err := readIntegrationChange(bound, receiptPath)
	if err != nil {
		return nil, err
	}
	changes, err := planSharedOwners(sharedRegistrationPlan{asset: managedAssetPlan{
		asset: asset, owners: []string{owner}, uninstall: uninstall, file: file, receipt: receipt,
	}})
	for i := range changes {
		if changes[i].Path == path && changes[i].Detail == "" {
			changes[i].Detail = "Agent Skill → " + path
		}
	}
	return changes, err
}

// Remove a previous host-specific copy only when its receipt proves ownership.
// Planning both destinations together keeps user edits and races visible to Apply.
func planMigratedSharedSkill(s Scope, root, legacyRoot, legacyID, owner string, uninstall bool) ([]Change, error) {
	if filepath.Clean(root) == filepath.Clean(legacyRoot) {
		return planSkillAsset(s, root, sharedSkillID, owner, skillContent, uninstall)
	}
	legacy, err := PlanManagedAssets(nativeSpecialScope(s, legacyRoot), []ManagedAsset{{
		Path: filepath.Join(legacyRoot, "obituary", "SKILL.md"), ID: legacyID, Owner: owner, Version: "1", Content: skillContent,
	}}, true)
	if err != nil {
		return nil, err
	}
	if !uninstall && PreservedError(legacy) != nil {
		return nil, fmt.Errorf("previous %s skill requires review: %w", owner, PreservedError(legacy))
	}
	shared, err := planSkillAsset(s, root, sharedSkillID, owner, skillContent, uninstall)
	if err != nil {
		return nil, err
	}
	return append(shared, legacy...), nil
}

// The old shared skill receipt belonged exclusively to Codex. Preserve that
// owner when another host joins, including recovery of interrupted upgrades.
func decodeLegacySharedSkill(data []byte, asset ManagedAsset) (assetReceipt, bool) {
	text := string(data)
	previous, hash := "", ""
	if body, ok := strings.CutPrefix(text, skillReceiptPrefix); ok {
		hash = strings.TrimSuffix(body, "\n")
	} else if body, ok := strings.CutPrefix(text, skillPendingPrefix); ok {
		parts := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
		if len(parts) != 2 {
			return assetReceipt{}, false
		}
		previous, hash = parts[0], parts[1]
	}
	valid := func(value string) bool {
		decoded, err := hex.DecodeString(value)
		return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
	}
	if !valid(hash) || previous != "" && previous != "absent" && !valid(previous) {
		return assetReceipt{}, false
	}
	return assetReceipt{Schema: 1, ID: asset.ID, File: "SKILL.md", Version: "1", Owners: []string{"codex"}, SHA256: hash, Previous: previous}, true
}
