package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanRoo follows Roo-Code/features/custom-instructions: separate native rules
// directories, with no verified runtime hook contract.
func PlanRoo(s Scope, _ Commands, uninstall bool) (changes []Change, err error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("rules are the only supported integration for Roo Code; --guard-rm has no verified runtime hook")
	}
	dir := filepath.Join(s.Project, ".roo")
	if s.Global {
		var err error
		dir, err = userdirs.RooDir()
		if err != nil {
			return nil, err
		}
		s = Scope{Project: dir, inspection: s.inspection}
	}
	s, err = boundedAgentScope(s, s.Project)
	if err != nil {
		return nil, err
	}
	ch, err := planAgentRule(s, filepath.Join(dir, "rules", "obituary.md"), uninstall)
	return []Change{ch}, err
}

func PlanRooMCP(s Scope, executable string, uninstall bool) (changes []Change, err error) {
	if s.Global {
		return manualAgentMCP("Roo Code", "the Roo Code MCP panel > Edit Global MCP (mcp_settings.json for the active editor/profile)", executable, uninstall)
	}
	s, err = boundedAgentScope(s, s.Project)
	if err != nil {
		return nil, err
	}
	ch, err := PlanMCPConfig(s, filepath.Join(s.Project, ".roo", "mcp.json"), MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
	return []Change{ch}, err
}

func init() {
	// PlanRoo installs rules, not runtime callbacks; unsupportedAgentSkill
	// leaves Skill discovery unverified (see docs/milestones/m4-agents.md).
	noHook := hook.Support{Detail: "rules only; no verified runtime hook"}
	RegisterAgent(Agent{Name: "roo", Method: RulesAndMCP, Plan: PlanRoo, PlanMCP: PlanRooMCP, PlanSkill: unsupportedAgentSkill("roo"), Capabilities: hook.Capabilities{
		DeleteGuard: noHook, WriteObservation: noHook, SessionStart: noHook, Stop: noHook, SessionEnd: noHook,
		Context: hook.Support{Level: hook.Limited, Detail: "global/project .roo/rules/obituary.md"},
		MCP:     hook.Support{Level: hook.Limited, Detail: "project .roo/mcp.json; global Edit Global MCP manual steps"},
		Skill:   hook.Support{Detail: "managed Skill discovery has not been verified; the installed rules describe Obituary; no Skill was installed"},
	}})
}
