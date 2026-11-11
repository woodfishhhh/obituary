package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
)

func planSharedAgentRules(s Scope, path, agent string, uninstall bool) (changes []Change, err error) {
	rule, err := planAgentRule(s, path, uninstall)
	if err != nil {
		return nil, err
	}
	asset := ManagedAsset{Path: path + ".obituary-owners", ID: "agent-rules", Owner: agent, Version: "1", Content: []byte("Obituary shared agent instructions\n")}
	p, err := readSharedAsset(s, asset)
	if err != nil {
		return nil, err
	}
	receipt := p.receipt
	start, _, err := markdownMarkedRange(string(rule.orig), blockBegin, blockEnd)
	if err != nil {
		return nil, err
	}
	requested := []string{agent}
	var checks []Change
	if start >= 0 && !receipt.existed {
		// Older project blocks did not record rules-only Hermes/Kimi owners.
		// Their absence cannot be proved from hooks: retain an unknown owner
		// even when a native Codex installation can also be positively found.
		if uninstall {
			rule.Action, rule.data, rule.delete = "preserve", nil, false
			rule.Detail = "legacy shared rules have no complete owner evidence; preserved"
			return []Change{rule, receipt}, nil
		}
		requested = append(requested, "legacy-unverified")
		hooksPath := filepath.Join(filepath.Dir(path), ".codex", "hooks.json")
		if s.Global {
			hooksPath = filepath.Join(filepath.Dir(path), "hooks.json")
		}
		hooks, err := readIntegrationChange(s, hooksPath)
		if err != nil {
			return nil, err
		}
		checks = append(checks, hooks)
		if hooks.existed {
			var config struct {
				Hooks map[string][]struct{ Hooks []json.RawMessage }
			}
			if err := json.Unmarshal(bytes.TrimPrefix(hooks.orig, []byte("\xef\xbb\xbf")), &config); err != nil {
				return nil, fmt.Errorf("legacy Codex owner: %w", err)
			}
			for _, groups := range config.Hooks {
				for _, group := range groups {
					for _, handler := range group.Hooks {
						if codexSpec("", "").IsOurs(handler) {
							requested = append(requested, "codex")
						}
					}
				}
			}
		}
	}
	changes, err = planSharedRegistration(s, asset, []Change{rule}, requested, uninstall)
	for i := range changes {
		if changes[i].Path == path {
			changes[i].phase = 2
			if uninstall {
				changes[i].phase = 0
			}
		} else if uninstall {
			changes[i].phase += 40 // retain the receipt until block removal succeeds
		}
	}
	return append(checks, changes...), err
}
