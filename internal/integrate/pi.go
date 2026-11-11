package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanPi installs the auto-discovered extension and the host's AGENTS.md block.
// Layout: badlogic/pi-mono packages/coding-agent/docs/extensions.md.
func PlanPi(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planPiHost(s, commands, "pi", uninstall)
}

func piHostDir(s Scope, agent string) (string, error) {
	if s.Global {
		return userdirs.AgentDir(agent)
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("%s installation needs an absolute project root", agent)
	}
	return filepath.Join(s.Project, "."+agent), nil
}

func planPiHost(s Scope, commands Commands, agent string, uninstall bool) ([]Change, error) {
	dir, err := piHostDir(s, agent)
	if err != nil {
		return nil, err
	}
	// Both hosts can resolve to the same agent root. The executable and opt-in
	// live in separate receipts so upgrades do not replace the other host's data.
	assets, err := planPlugin(s, dir, agent, "pi-omp-extension", "plugins/pi/index.ts", filepath.Join("extensions", "obituary.ts"), commands.Executable, uninstall)
	if err != nil {
		return nil, err
	}
	rules := filepath.Join(s.Project, "AGENTS.md")
	if s.Global {
		rules = filepath.Join(dir, "AGENTS.md")
	}
	block, err := planPluginRules(s, rules, agent, uninstall)
	if err != nil {
		return nil, err
	}
	return OrderManagedInstallation(assets, []Change{block}, uninstall)
}

func planPiMCP(s Scope, executable, agent string, uninstall bool) ([]Change, error) {
	dir, err := piHostDir(s, agent)
	if err != nil {
		return nil, err
	}
	// Pi and OMP may share a root. The entry has its own owner receipt, so
	// uninstalling one host cannot unregister a server used by the other.
	path := filepath.Join(dir, "mcp.json")
	return planPluginMCP(s, path, agent, executable, uninstall)
}

func PlanPiMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	return planPiMCP(s, executable, "pi", uninstall)
}

func unsupportedPluginSkill(agent string) func(Scope, bool) ([]Change, error) {
	return func(Scope, bool) ([]Change, error) {
		return nil, fmt.Errorf("%s: automatic Skill installation is not verified; existing skills are left untouched; the installed AGENTS.md block provides Obituary instructions", agent)
	}
}

func PlanPiSkill(s Scope, uninstall bool) ([]Change, error) {
	return PlanSharedSkill(s, "pi", uninstall)
}

func init() {
	host, _ := hook.Lookup("pi")
	caps := host.Capabilities
	caps.Skill = hook.Support{Level: hook.Supported, Detail: "Shared .agents/skills at user and project scope"}
	caps.DeleteGuard.Detail = "tool_call block/reason; requires OBIT_PI_LOCAL=1 and OBIT_PI_SHELL=bash or powershell after verifying the backend"
	caps.WriteObservation.Detail = "Successful local write/edit tool_result; requires OBIT_PI_LOCAL=1; passive content delivery; apply_patch is skipped"
	caps.SessionStart.Detail = "session_start uses stable sessionManager id and passive sendMessage with triggerTurn:false; local backend attestation required"
	caps.Stop.Detail = "agent_end and agent_settled map to silent Stop; local backend attestation required"
	caps.SessionEnd.Detail = "session_shutdown maps to silent SessionEnd with a 1.2-second subprocess budget"
	caps.Context.Detail = "Passive tool_result content and session_start sendMessage"
	caps.MCP = hook.Support{Level: hook.Supported, Detail: "Native user/project mcp.json with shared Pi/OMP ownership"}
	RegisterAgent(Agent{Name: "pi", Method: Plugin, Capabilities: caps, Plan: PlanPi, PlanMCP: PlanPiMCP, PlanSkill: PlanPiSkill})
}
