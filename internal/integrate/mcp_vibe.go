package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Vibe uses [[mcp_servers]] in the user or trusted project config.toml.
func PlanVibeMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	root, err := vibeRoot(s)
	if err != nil {
		return nil, err
	}
	bounded := nativeSpecialScope(s, root)
	path := filepath.Join(root, "config.toml")
	before, base, err := nativeSpecialTOMLBase(bounded, path, "vibe-mcp")
	if err != nil {
		return nil, err
	}
	if servers, exists := base["mcp_servers"]; exists {
		entries, ok := servers.([]any)
		if !ok {
			return nil, fmt.Errorf("%s: mcp_servers must be an array", path)
		}
		for _, item := range entries {
			e, ok := item.(map[string]any)
			if !ok || e["name"] == "obituary" {
				return nil, fmt.Errorf("%s: unmarked obituary MCP entry; review it manually", path)
			}
		}
	}
	if !uninstall && !isObitMCP(executable, []string{"mcp"}) {
		return nil, fmt.Errorf("invalid Obituary MCP executable")
	}
	content, err := toml.Marshal(map[string]any{"mcp_servers": []map[string]any{{"name": "obituary", "transport": "stdio", "command": executable, "args": []string{"mcp"}}}})
	if err != nil {
		return nil, err
	}
	ch, err := PlanTOMLBlock(bounded, path, TOMLBlock{ID: "vibe-mcp", Content: content, Owns: ownsVibeMCP,
		Refresh: func(block []byte) ([]byte, error) {
			return RefreshTOMLString(block, []string{"mcp_servers"}, "command", executable)
		}}, uninstall)
	if err != nil {
		return nil, err
	}
	if err := nativeSpecialOriginal(ch, before); err != nil {
		return nil, err
	}
	return []Change{ch}, nil
}

func ownsVibeMCP(full map[string]any) bool {
	entries, ok := full["mcp_servers"].([]any)
	if !ok || len(full) != 1 || len(entries) != 1 {
		return false
	}
	entry, ok := entries[0].(map[string]any)
	if !ok || entry["name"] != "obituary" || entry["transport"] != "stdio" {
		return false
	}
	raw, err := marshal(entry)
	return err == nil && (CommandArgsMCP{}).Owned(raw)
}
