package integrate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanKimi targets MoonshotAI/kimi-cli 9ab1286b Beta hooks. Project instructions
// use AGENTS.md because project config is not auto-loaded. Deferred or
// unverifiable native callbacks stay unregistered.
func PlanKimi(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("delete guard for Kimi is unavailable: the Beta payload does not attest kaos runtime locality")
	}
	if !s.Global {
		return planSharedAgentRules(s, filepath.Join(s.Project, "AGENTS.md"), "kimi", uninstall)
	}
	root, err := userdirs.KimiDir()
	if err != nil {
		return nil, err
	}
	if !uninstall && (!OwnedHookCommand(commands.Native, "kimi", nil) || strings.HasPrefix(strings.TrimSpace(commands.Native), "& ") || strings.ContainsAny(commands.Native, "%!")) {
		return nil, fmt.Errorf("hooks for Kimi on Windows require a literal cmd.exe-compatible obit hook kimi command")
	}
	bounded := nativeSpecialScope(s, root)
	path := filepath.Join(root, "config.toml")
	ch, err := kimiAdapter.plan(bounded, path, commands.Native, uninstall)
	if err != nil {
		return nil, err
	}
	return []Change{ch}, nil
}

func ownsKimiHooks(full map[string]any) bool {
	entries, ok := full["hooks"].([]any)
	if !ok || len(full) != 1 || len(entries) != 2 {
		return false
	}
	seen := map[string]bool{}
	for _, item := range entries {
		e, ok := item.(map[string]any)
		event, _ := e["event"].(string)
		command, _ := e["command"].(string)
		if !ok || seen[event] || event != "Stop" && event != "SessionEnd" || !OwnedHookCommand(command, "kimi", nil) {
			return false
		}
		seen[event] = true
	}
	return true
}

func PlanKimiSkill(s Scope, uninstall bool) ([]Change, error) {
	root := filepath.Join(s.Project, ".kimi", "skills")
	if s.Global {
		var err error
		root, err = userdirs.KimiSkillsDir()
		if err != nil {
			return nil, err
		}
	}
	shared := filepath.Join(s.Project, ".agents", "skills")
	if s.Global {
		var err error
		shared, err = userdirs.AgentSkillsDir()
		if err != nil {
			return nil, err
		}
	}
	return planMigratedSharedSkill(s, shared, root, "kimi-skill", "kimi", uninstall)
}

func init() {
	host, _ := hook.Lookup("kimi")
	caps := host.Capabilities
	caps.Skill = hook.Support{Level: hook.Limited, Detail: "Shared .agents/skills; user ~/.config/agents/skills takes precedence unless merge_all_available_skills is enabled"}
	RegisterAgent(Agent{Name: "kimi", Method: NativeHooks, Capabilities: caps, Plan: PlanKimi, PlanMCP: PlanKimiMCP, PlanSkill: PlanKimiSkill})
}

var kimiAdapter = tomlHookAdapter{
	agent: "kimi", id: "kimi-hooks",
	conflict: "unmarked Kimi Obituary hook; review it manually",
	events:   []map[string]any{{"event": "Stop", "timeout": 5}, {"event": "SessionEnd", "timeout": 5}},
	owns:     ownsKimiHooks,
	detail:   "Kimi CLI Beta: silent Stop/SessionEnd only; no verified start baseline, write observation, delete guard or context delivery; project AGENTS.md and --skill provide instructions",
}
