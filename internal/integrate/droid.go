package integrate

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func droidDirectory(s Scope) (string, error) {
	if s.Global {
		return userdirs.DroidDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("droid requires an absolute project directory")
	}
	return filepath.Join(s.Project, ".factory"), nil
}

// Standalone/legacy hooks have events at the root; settings.json nests them.
// See docs.factory.com/harness/hooks. Droid's Windows hook runner uses cmd.exe,
// so Commands.Native is kept intact for the host's command parsing.
var droidAdapter = hookAdapter{
	guardReason: "delete guard for Droid is unsupported: Execute does not identify its shell/backend; no PreToolUse hook was installed",
	config: HookConfig{Agent: "droid", Shape: RootHooks, EventArgs: []string{},
		Events: []HookRegistration{{Event: "SessionStart"}, {Event: "Stop"}, {Event: "SessionEnd"}}},
	removeEvents: []HookRegistration{{Event: "PreToolUse"}, {Event: "PostToolUse"}},
	handler:      nativeHookHandler(map[string]any{"type": "command", "timeout": 5}, false),
}

func PlanDroid(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if err := rejectUnsupportedGuard(s, uninstall, droidAdapter.guardReason); err != nil {
		return nil, err
	}
	dir, err := droidDirectory(s)
	if err != nil {
		return nil, err
	}
	before, target, err := droidHookFiles(s, dir, uninstall)
	if err != nil {
		return nil, err
	}
	spec, err := droidAdapter.hooks(s, commands, uninstall)
	if err != nil {
		return nil, err
	}
	changes, err := planDroidHooks(s, before, target, spec, uninstall)
	if err != nil {
		return nil, err
	}
	// Factory searches .factory/ for instructions in both scopes.
	rules, err := planHookRules(s, nil, filepath.Join(dir, "AGENTS.md"), uninstall)
	if err != nil {
		return nil, err
	}
	return append(changes, rules...), nil
}

func droidHookFiles(s Scope, dir string, uninstall bool) ([]Change, int, error) {
	paths := []string{filepath.Join(dir, "hooks.json"), filepath.Join(dir, "hooks", "hooks.json"), filepath.Join(dir, "settings.json")}
	before := make([]Change, len(paths))
	var settingsHooks *object
	for i, path := range paths {
		ch, err := readIntegrationChange(s, path)
		if err != nil {
			return nil, 0, err
		}
		if err := nonemptyConfig(ch); err != nil {
			return nil, 0, err
		}
		data, err := configJSON(ch.orig, ConfigJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", path, err)
		}
		before[i] = ch
		if i == 2 {
			hooks, err := configMember(data, []string{"hooks"})
			if err != nil {
				return nil, 0, err
			}
			settingsHooks, err = parseObject(hooks)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	target := 0
	if !before[0].existed && before[1].existed {
		target = 1
	}
	if !settingsHooks.empty() {
		if !uninstall && (before[0].existed || before[1].existed) {
			return nil, 0, fmt.Errorf("hook declarations for Droid coexist in %s and %s; loader precedence differs by version: consolidate the effective events with Droid /hooks into one file before retrying; all files left untouched", paths[target], paths[2])
		}
		target = 2
	}
	return before, target, nil
}

func planDroidHooks(s Scope, before []Change, target int, spec HookConfig, uninstall bool) ([]Change, error) {
	var changes []Change
	for i, old := range before {
		if !uninstall && i != target {
			changes = append(changes, old)
			continue
		}
		spec.Shape = RootHooks
		if i == 2 {
			spec.Shape = GroupedHooks
		}
		change, err := PlanHookConfig(s, old.Path, spec, uninstall)
		if err != nil {
			return nil, err
		}
		if change.existed != old.existed || !bytes.Equal(change.orig, old.orig) {
			return nil, fmt.Errorf("%s changed while planning Droid hooks; retry", old.Path)
		}
		change.phase = 1
		if uninstall {
			change.phase = 0
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func PlanDroidMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	dir, err := droidDirectory(s)
	if err != nil {
		return nil, err
	}
	return planNonemptyMCP(s, filepath.Join(dir, "mcp.json"), MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{Type: "stdio"}}, executable, uninstall)
}

func PlanDroidSkill(s Scope, uninstall bool) ([]Change, error) {
	dir, err := droidDirectory(s)
	if err != nil {
		return nil, err
	}
	return PlanManagedAssets(s, []ManagedAsset{{Path: filepath.Join(dir, "skills", "obituary", "SKILL.md"), ID: "droid-obituary-skill", Owner: "droid", Version: "1", Content: skillContent}}, uninstall)
}

func init() {
	host, _ := hook.Lookup("droid")
	capabilities := host.Capabilities
	capabilities.MCP = hook.Support{Level: hook.Supported, Detail: ".factory/mcp.json; local stdio"}
	capabilities.Skill = hook.Support{Level: hook.Supported, Detail: ".factory/skills, including FACTORY_HOME_OVERRIDE"}
	RegisterAgent(Agent{Name: "droid", Method: NativeHooks, Capabilities: capabilities, Plan: PlanDroid, PlanMCP: PlanDroidMCP, PlanSkill: PlanDroidSkill})
}
