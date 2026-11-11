package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func openCodeDir(s Scope) (string, error) {
	if s.Global {
		return userdirs.OpenCodeDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("OpenCode installation needs an absolute project root")
	}
	return filepath.Join(s.Project, ".opencode"), nil
}

// PlanOpenCode installs the host's auto-discovered V1/V2 plugin layout.
// V1 requires >=1.18.29 for the dual object export.
func PlanOpenCode(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	dir, err := openCodeDir(s)
	if err != nil {
		return nil, err
	}
	assets, err := planPlugin(s, dir, "opencode", "opencode-plugin", "plugins/opencode/index.ts", filepath.Join("plugins", "obituary.ts"), commands.Executable, uninstall)
	if err != nil {
		return nil, err
	}
	rules := filepath.Join(s.Project, "AGENTS.md")
	if s.Global {
		rules = filepath.Join(dir, "AGENTS.md")
	}
	block, err := planPluginRules(s, rules, "opencode", uninstall)
	if err != nil {
		return nil, err
	}
	return OrderManagedInstallation(assets, []Change{block}, uninstall)
}

func openCodeMCPPath(s Scope) (string, error) {
	dir, err := openCodeDir(s)
	if err != nil {
		return "", err
	}
	var paths []string
	if s.Global {
		if override, err := userdirs.OpenCodeConfigOverride(); err != nil {
			return "", err
		} else if override != "" {
			return override, nil
		}
	} else {
		paths = append(paths, filepath.Join(s.Project, "opencode.json"), filepath.Join(s.Project, "opencode.jsonc"))
	}
	paths = append(paths, filepath.Join(dir, "opencode.json"), filepath.Join(dir, "opencode.jsonc"))
	var existing []string
	for _, path := range paths {
		ch, err := readIntegrationChange(s, path)
		if err != nil {
			return "", err
		}
		if ch.existed {
			existing = append(existing, path)
		}
	}
	if len(existing) > 1 {
		return "", fmt.Errorf("OpenCode has multiple configuration layers %v; select the intended MCP layer manually to avoid shadowing", existing)
	}
	if len(existing) == 1 {
		return existing[0], nil
	}
	if !s.Global {
		return filepath.Join(s.Project, "opencode.json"), nil
	}
	return filepath.Join(dir, "opencode.json"), nil
}

func PlanOpenCodeMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	path, err := openCodeMCPPath(s)
	if err != nil {
		return nil, err
	}
	if err := s.checkInside(filepath.Dir(path), path, path+".bak"); err != nil {
		return nil, err
	}
	ch, err := PlanMCPConfig(s, path, MCPConfig{Format: ConfigJSONC, ServersPath: []string{"mcp"}, Codec: CommandArrayMCP{}}, executable, uninstall)
	return []Change{ch}, err
}

func PlanOpenCodeSkill(s Scope, uninstall bool) ([]Change, error) {
	return PlanSharedSkill(s, "opencode", uninstall)
}

func init() {
	host, _ := hook.Lookup("opencode")
	caps := host.Capabilities
	caps.Skill = hook.Support{Level: hook.Supported, Detail: "Shared .agents/skills at user and project scope"}
	caps.DeleteGuard.Detail = "V1 >=1.18.29 error / V2 Tool.Error; OBIT_OPENCODE_LOCAL=1 and OBIT_OPENCODE_SHELL=bash or powershell require verified local backend"
	caps.WriteObservation.Detail = "Successful write/edit result only; V1 output or V2 result content; local backend opt-in required; apply_patch is skipped"
	caps.SessionStart.Detail = "V1 session.created / V2 first prompt are observation-only; processing awaits deferred-context support"
	caps.Stop.Detail = "V1 session.idle / V2 experimental.idle map to silent Stop"
	caps.SessionEnd.Detail = "V1 session.deleted handles persistent-session deletion; V2 uses idle cleanup only"
	caps.Context.Detail = "V1 tool output / V2 result content; startup context awaits deferred delivery"
	caps.MCP = hook.Support{Level: hook.Supported, Detail: "Native JSONC mcp command array; ambiguous layers require manual selection"}
	RegisterAgent(Agent{Name: "opencode", Method: Plugin, Capabilities: caps, Plan: PlanOpenCode, PlanMCP: PlanOpenCodeMCP, PlanSkill: PlanOpenCodeSkill})
}
