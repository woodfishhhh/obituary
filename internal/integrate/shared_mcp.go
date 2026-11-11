package integrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Every installer of mcpServers.obituary uses the same receipt at the actual
// configuration path. Historical Pi/OMP and CodeBuddy receipt formats remain
// readable; ownership never follows the family of the current installer.
func planSharedMCP(s Scope, path, agent string, spec MCPConfig, executable string, uninstall bool) ([]Change, error) {
	mcp, err := PlanMCPConfig(s, path, spec, executable, uninstall)
	if err != nil {
		return nil, err
	}
	actual, resolveErr := filepath.EvalSymlinks(path)
	if errors.Is(resolveErr, os.ErrNotExist) {
		actual = path
	} else if resolveErr != nil {
		return nil, resolveErr
	}
	mcp.Path = actual
	asset := ManagedAsset{Path: actual + ".obituary-owners", ID: "pi-omp-mcp", Owner: agent, Version: "1", Content: []byte("Obituary Pi/OMP MCP registration\n"), receiptSchema: 2}
	p, err := readSharedAsset(s, asset)
	if err != nil {
		return nil, err
	}
	receipt := p.receipt
	legacy := asset
	legacy.ID, legacy.Content = "codebuddy-mcp", []byte("Obituary shared registration: codebuddy-mcp\n")
	if _, valid := decodeAssetReceipt(receipt.orig, legacy); valid {
		asset = legacy
	}
	if !uninstall && !receipt.existed {
		format := spec.Format
		if format == "" {
			format = ConfigJSON
		}
		data, err := configJSON(mcp.orig, format)
		if err != nil {
			return nil, err
		}
		entry, err := configMember(data, []string{"mcpServers", "obituary"})
		if err != nil {
			return nil, err
		}
		if entry != nil {
			return nil, fmt.Errorf("%s: existing obituary MCP has no shared owner receipt; retained", path)
		}
	}
	changes, err := planSharedRegistration(s, asset, []Change{mcp}, []string{agent}, uninstall)
	if err != nil {
		return nil, err
	}
	// Keep the public planner's primary configuration first; dependency phases
	// still commit owner evidence before adding the registration.
	return append(changes[len(changes)-1:], changes[:len(changes)-1]...), nil
}
