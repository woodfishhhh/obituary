package integrate

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

//go:embed plugins/cline/obituary.ts
var clinePlugin []byte

// PlanCline targets SDK/CLI/Kanban plugins (docs.cline.bot/customization/plugins).
// The VS Code and JetBrains clients currently receive rules only from these paths.
func PlanCline(s Scope, commands Commands, uninstall bool) (changes []Change, err error) {
	dir := filepath.Join(s.Project, ".cline")
	bound := s
	if s.Global {
		var err error
		dir, err = userdirs.ClineDir()
		if err != nil {
			return nil, err
		}
		bound = Scope{Project: dir, GuardRM: s.GuardRM, inspection: s.inspection}
	}
	bound, err = boundedAgentScope(bound, bound.Project)
	if err != nil {
		return nil, err
	}
	rulePath := filepath.Join(dir, "rules", "obituary.md")
	if !s.Global {
		legacy := filepath.Join(s.Project, ".clinerules")
		if err := s.checkInside(s.Project, legacy); err != nil {
			return nil, err
		}
		info, err := os.Lstat(legacy)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err == nil && info.Mode().IsRegular() {
			rulePath = legacy
		}
	}
	rule, err := planAgentRule(bound, rulePath, uninstall)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "plugins", "obituary.ts")
	payload := clinePlugin
	if !uninstall {
		payload, err = pluginPayload(bound, path, payload, commands.Executable)
		if err != nil {
			return nil, err
		}
	}
	assets, err := PlanManagedAssets(bound, []ManagedAsset{{Path: path, ID: "cline-plugin", Owner: "cline", Version: "1", Content: payload}}, uninstall)
	for i := range assets {
		if assets[i].Path == path {
			assets[i].Detail = "Cline SDK/CLI/Kanban plugin; guard and local observation require OBIT_CLINE_LOCAL_SHELL=bash or powershell after verifying the executor; VS Code/JetBrains plugins are unsupported"
		}
	}
	return append(assets, rule), err
}

func PlanClineMCP(s Scope, executable string, uninstall bool) (changes []Change, err error) {
	if !s.Global {
		return manualAgentMCP("Cline project scope", "Cline Settings > MCP Servers > Configure MCP Servers (user settings; project discovery is unverified)", executable, uninstall)
	}
	dir, err := userdirs.ClineDataDir()
	if err != nil {
		return nil, err
	}
	bound, err := boundedAgentScope(s, dir)
	if err != nil {
		return nil, err
	}
	ch, err := PlanMCPConfig(bound, filepath.Join(dir, "settings", "cline_mcp_settings.json"), MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
	return []Change{ch}, err
}

func init() {
	host, _ := hook.Lookup("cline")
	RegisterAgent(Agent{Name: "cline", Method: Plugin, Capabilities: host.Capabilities, Plan: PlanCline, PlanMCP: PlanClineMCP, PlanSkill: unsupportedAgentSkill("cline")})
}
