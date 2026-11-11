package integrate

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Bundle layout is documented at antigravity.google/docs/plugins. Native
// commands keep installation independent of a TS/Python runtime.
//
//go:embed plugins/antigravity/plugin.json
var antigravityManifest []byte

//go:embed plugins/antigravity/hooks.json
var antigravityHooks []byte

func antigravityRoot(s Scope) (string, error) {
	if s.Global {
		return userdirs.AntigravityDir()
	}
	return filepath.Join(s.Project, ".agents"), nil
}

func PlanAntigravity(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("delete guard for Antigravity is unavailable: the native payload does not identify a local execution shell")
	}
	root, err := antigravityRoot(s)
	if err != nil {
		return nil, err
	}
	if !uninstall && !OwnedHookCommand(commands.Native, "antigravity", nil) {
		return nil, fmt.Errorf("invalid Antigravity hook command")
	}
	hooks := bytes.Clone(antigravityHooks)
	for marker, event := range map[string]string{"OBIT_SESSION_START": "SessionStart", "OBIT_STOP": "Stop"} {
		command, err := marshal(commands.Native + " --event " + event)
		if err != nil {
			return nil, err
		}
		hooks = bytes.ReplaceAll(hooks, []byte(`"`+marker+`"`), command)
	}
	dir := filepath.Join(root, "plugins", "obituary")
	assets := []ManagedAsset{
		{Path: filepath.Join(dir, "hooks.json"), ID: "antigravity-hooks", Owner: "antigravity", Version: "1", Content: hooks},
		{Path: filepath.Join(dir, "rules", "AGENTS.md"), ID: "antigravity-rules", Owner: "antigravity", Version: "1", Content: []byte(Awareness)},
		{Path: filepath.Join(dir, "plugin.json"), ID: "antigravity-plugin", Owner: "antigravity", Version: "1", Content: antigravityManifest, Activation: true},
	}
	changes, err := PlanManagedAssets(nativeSpecialScope(s, root), assets, uninstall)
	if err != nil {
		return nil, err
	}
	// An edited activation manifest may still refer to every payload file.
	// Retain the entire bundle until the user resolves that manifest conflict.
	if uninstall {
		manifest := filepath.Join(dir, "plugin.json")
		for _, ch := range changes {
			if ch.Action == "preserve" && (ch.Path == manifest || ch.Path == ManagedAssetReceiptPath(manifest)) {
				for i := range changes {
					if changes[i].Action == "remove" {
						changes[i].Action, changes[i].Detail = "preserve", "bundle retained because activation manifest ownership is unverified"
					}
				}
				break
			}
		}
	}
	for i := range changes {
		if changes[i].Action != "preserve" {
			changes[i].Detail = "Antigravity native bundle; SessionStart/Stop require one local workspace; PostToolUse deferred delivery unavailable; review enablement in /plugins"
		}
	}
	return changes, nil
}

func PlanAntigravityMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	root, err := antigravityRoot(s)
	if err != nil {
		return nil, err
	}
	ch, err := PlanMCPConfig(nativeSpecialScope(s, root), filepath.Join(root, "mcp_config.json"), MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
	return []Change{ch}, err
}

func init() {
	host, _ := hook.Lookup("antigravity")
	RegisterAgent(Agent{Name: "antigravity", Method: NativeHooks, Capabilities: host.Capabilities, Plan: PlanAntigravity, PlanMCP: PlanAntigravityMCP})
}
