package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
)

// PlanWindsurf uses .windsurfrules for project instruction compatibility.
// Current official pages redirect to Devin; those hooks are a different target.
func PlanWindsurf(s Scope, _ Commands, uninstall bool) (changes []Change, err error) {
	if s.Global {
		if uninstall {
			return nil, manualf("Windsurf global rules: open Cascade > Customizations > Rules > Global; remove only the standalone block from %s through %s; retain all surrounding rules; automatic removal is unavailable", blockBegin, blockEnd)
		}
		return nil, manualf("Windsurf global rules: open Cascade > Customizations > Rules > Global; append the following block while retaining existing rules; the active client's global file location requires manual verification and no file was installed:\n%s\n%s%s\n", blockBegin, Awareness, blockEnd)
	}
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("project rules are the only supported integration for Windsurf; --guard-rm has no verified legacy hook protocol")
	}
	s, err = boundedAgentScope(s, s.Project)
	if err != nil {
		return nil, err
	}
	ch, err := planAgentRule(s, filepath.Join(s.Project, ".windsurfrules"), uninstall)
	return []Change{ch}, err
}

func PlanWindsurfMCP(_ Scope, executable string, uninstall bool) ([]Change, error) {
	return manualAgentMCP("Windsurf", "Cascade > MCP Servers > Configure > View raw config for the active client (verify Windsurf versus Devin before editing)", executable, uninstall)
}

func init() {
	// PlanWindsurf only installs project rules; the legacy hook protocol and
	// managed Skill discovery remain unverified (docs/milestones/m4-agents.md).
	noHook := hook.Support{Detail: "legacy Windsurf project rules only"}
	RegisterAgent(Agent{Name: "windsurf", Method: RulesAndMCP, Plan: PlanWindsurf, PlanMCP: PlanWindsurfMCP, PlanSkill: unsupportedAgentSkill("windsurf"), Capabilities: hook.Capabilities{
		DeleteGuard: noHook, WriteObservation: noHook, SessionStart: noHook, Stop: noHook, SessionEnd: noHook,
		Context: hook.Support{Level: hook.Limited, Detail: "project .windsurfrules; global rules manual"},
		MCP:     hook.Support{Level: hook.Limited, Detail: "manual raw config in the active client; Devin is a separate target"},
		Skill:   hook.Support{Detail: "The installed rules describe Obituary; Skill installation is not implemented for this agent"},
	}})
}
