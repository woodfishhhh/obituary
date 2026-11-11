package integrate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func vibeRoot(s Scope) (string, error) {
	if s.Global {
		return userdirs.VibeDir()
	}
	return filepath.Join(s.Project, ".vibe"), nil
}

// PlanVibe uses the host's native TOML hooks.
// Current official project discovery also supports trusted .vibe/hooks.toml.
func PlanVibe(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("delete guard for Vibe is unavailable: native payload does not attest shell/runtime locality")
	}
	root, err := vibeRoot(s)
	if err != nil {
		return nil, err
	}
	command := commands.Native
	if !uninstall {
		words, ok := shellWords(command)
		if !ok || !OwnedHookCommand(command, "vibe", nil) {
			return nil, fmt.Errorf("invalid Vibe hook command")
		}
		// Vibe launches through Git Bash or PowerShell. A space-free literal
		// works in both; quoted executables need incompatible invocation syntax.
		// Keep this explicit until the host exposes its selected hook launcher.
		if strings.ContainsAny(words[0], " \t&|;<>()`$\"'!%") {
			return nil, fmt.Errorf("hook launcher for Vibe can be Git Bash or PowerShell: use a space-free obit.exe path without shell metacharacters, or a verified obit on PATH; current executable requires manual launcher-specific quoting")
		}
		command = filepath.ToSlash(words[0]) + " hook vibe"
	}
	bounded := nativeSpecialScope(s, root)
	path := filepath.Join(root, "hooks.toml")
	ch, err := vibeAdapter.plan(bounded, path, command, uninstall)
	if err != nil {
		return nil, err
	}
	changes := []Change{ch}
	if !s.Global {
		rules, err := planSharedAgentRules(s, filepath.Join(s.Project, "AGENTS.md"), "vibe", uninstall)
		return append(changes, rules...), err
	}
	// The prompt is deliberately not selected for the user. Vibe's selected
	// system_prompt_id may carry unrelated behavior that must stay intact.
	assets, err := PlanManagedAssets(bounded, []ManagedAsset{{Path: filepath.Join(root, "prompts", "obituary.md"), ID: "vibe-prompt", Owner: "vibe", Version: "1", Content: []byte(Awareness)}}, uninstall)
	for i := range assets {
		if assets[i].Action != "preserve" {
			assets[i].Detail = "optional prompt; manual activation: review prompts/obituary.md and select system_prompt_id = \"obituary\" in Vibe config.toml, or incorporate its text into the selected prompt"
		}
	}
	return append(changes, assets...), err
}

func ownsVibeHooks(full map[string]any) bool {
	entries, ok := full["hooks"].([]any)
	if !ok || len(full) != 1 || len(entries) != 2 {
		return false
	}
	seen := map[string]bool{}
	for _, item := range entries {
		e, ok := item.(map[string]any)
		name, _ := e["name"].(string)
		kind, _ := e["type"].(string)
		command, _ := e["command"].(string)
		if !ok || seen[name] || !OwnedHookCommand(command, "vibe", nil) || ((name != "obituary-write" || kind != "post_tool") && (name != "obituary-stop" || kind != "post_agent")) {
			return false
		}
		seen[name] = true
	}
	return true
}

func PlanVibeSkill(s Scope, uninstall bool) ([]Change, error) {
	root, err := vibeRoot(s)
	if err != nil {
		return nil, err
	}
	return PlanManagedAssets(nativeSpecialScope(s, root), []ManagedAsset{{Path: filepath.Join(root, "skills", "obituary", "SKILL.md"), ID: "vibe-skill", Owner: "vibe", Version: "1", Content: skillContent}}, uninstall)
}

func init() {
	host, _ := hook.Lookup("vibe")
	RegisterAgent(Agent{Name: "vibe", Method: NativeHooks, Capabilities: host.Capabilities, Plan: PlanVibe, PlanMCP: PlanVibeMCP, PlanSkill: PlanVibeSkill})
}

var vibeAdapter = tomlHookAdapter{
	agent: "vibe", id: "vibe-hooks", reservedNames: []string{"obituary-write", "obituary-stop"},
	conflict: "conflicting unmarked Vibe hook; review it manually",
	events: []map[string]any{
		{"name": "obituary-write", "type": "post_tool", "match": "re:^(file_system\\.)?(write_file|edit)$", "timeout": 5, "strict": false},
		{"name": "obituary-stop", "type": "post_agent", "match": "*", "timeout": 5, "strict": false},
	},
	owns:   ownsVibeHooks,
	detail: "Vibe post_tool write observation and silent post_agent; no start/end snapshot; project hooks require trust",
}
