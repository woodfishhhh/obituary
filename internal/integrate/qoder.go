package integrate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func qoderDirectory(s Scope, agent string) (string, error) {
	if s.Global {
		return userdirs.AgentDir(agent)
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("%s requires an absolute project directory", agent)
	}
	leaf := ".qoder"
	if agent == "lingma" {
		leaf = ".lingma"
	}
	return filepath.Join(s.Project, leaf), nil
}

// Qoder shares native grouped hooks with the IDE. The shared checked editor
// preserves configuration and ownership during registration.
func PlanQoder(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planQoder(s, commands, "qoder", uninstall)
}

func PlanQoderCN(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planQoder(s, commands, "qoder-cn", uninstall)
}

func planQoder(s Scope, commands Commands, agent string, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("%s delete guard is limited: the hook does not identify its execution shell/backend; no PreToolUse hook installed", agent)
	}
	dir, err := qoderDirectory(s, agent)
	if err != nil {
		return nil, err
	}
	bounded := nativeSpecialScope(s, dir)
	protocol := agent
	if !s.Global {
		protocol = "qoder" // one invocation even when both editions are installed
	}
	command := commands.Native
	if !uninstall {
		if !OwnedHookCommand(command, agent, nil) {
			return nil, fmt.Errorf("invalid %s hook command", agent)
		}
		command = strings.TrimSuffix(strings.TrimSpace(command), "hook "+agent) + "hook " + protocol
	}
	registrations, err := qoderHookLayers(s, dir, protocol, command, uninstall)
	if err != nil {
		return nil, err
	}
	if !s.Global {
		registrations, err = qoderSharedRegistration(bounded, filepath.Join(dir, "obituary-hooks.owners"), "qoder-hooks", agent, registrations, uninstall)
		if err != nil {
			return nil, err
		}
	}
	// Both current CLI memory references document trigger:always_on. IDE rule
	// metadata is never rewritten: managed-asset checks preserve edited owned files.
	rules, err := PlanManagedAssets(bounded, []ManagedAsset{{
		Path: filepath.Join(dir, "rules", "obituary.md"), ID: "qoder-obituary-rule", Owner: agent, Version: "1",
		Content: []byte("---\ntrigger: always_on\n---\n\n" + Awareness),
	}}, uninstall)
	if err != nil {
		return nil, err
	}
	// Shared registration already has receipt transactions; preserve its
	// phases instead of passing those transactions as plain registrations.
	return append(rules, registrations...), nil
}

// IDE hooks merge all layers; CLI settings.local.json has highest file
// precedence. Install in that file when present, removing only our duplicate
// registrations from lower project layers. User hooks remain in their order.
func qoderHookLayers(s Scope, dir, agent, command string, uninstall bool) ([]Change, error) {
	bounded := nativeSpecialScope(s, dir)
	paths := []string{filepath.Join(dir, "settings.json")}
	if !s.Global {
		paths = append(paths, filepath.Join(dir, "settings.local.json"))
	}
	before := make([]Change, len(paths))
	active := 0
	for i, path := range paths {
		var err error
		before[i], err = qoderConfig(bounded, path)
		if err != nil {
			return nil, err
		}
		if before[i].existed {
			active = i
		}
	}
	adapter := qoderHookAdapter(agent)
	spec, err := adapter.hooks(s, Commands{Native: command}, uninstall)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for i, path := range paths {
		if !uninstall && agent == "qoder" && !s.Global {
			if err := qoderProjectConflict(before[i]); err != nil {
				return nil, err
			}
		}
		ch, err := PlanHookConfig(bounded, path, spec, uninstall || i != active)
		if err != nil {
			return nil, err
		}
		if err := nativeSpecialOriginal(ch, before[i]); err != nil {
			return nil, err
		}
		ch.phase = 20
		if uninstall {
			ch.phase = -40
		}
		ch.Detail = agent + ": successful writes are deferred to a later prompt; guard unavailable; restart the IDE and review CLI folder trust; explicit --settings can override these files"
		changes = append(changes, ch)
	}
	return changes, nil
}

func qoderConfig(s Scope, path string) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	if err := nonemptyConfig(ch); err != nil {
		return ch, err
	}
	_, err = configJSON(ch.orig, ConfigJSONC)
	return ch, err
}

// The managed-asset receipt records the shared registration's owner set.
// Uninstall can remove a shared registration only for its last owner.
func qoderSharedRegistration(s Scope, path, id, agent string, registration []Change, uninstall bool) ([]Change, error) {
	asset := ManagedAsset{Path: path, ID: id, Owner: agent, Version: "1", Content: []byte("Obituary " + id + " shared registration\n")}
	return planSharedRegistration(s, asset, registration, []string{agent}, uninstall)
}

func PlanQoderMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	return planQoderMCP(s, executable, "qoder", uninstall)
}

func PlanQoderCNMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	return planQoderMCP(s, executable, "qoder-cn", uninstall)
}

func planQoderMCP(s Scope, executable, agent string, uninstall bool) ([]Change, error) {
	dir, err := qoderDirectory(s, agent)
	if err != nil {
		return nil, err
	}
	bounded := nativeSpecialScope(s, dir)
	paths := []string{filepath.Join(dir, "settings.json")}
	if !s.Global {
		// Same-name priority: project settings < .mcp.json < local settings.
		paths = append(paths, filepath.Join(s.Project, ".mcp.json"), filepath.Join(dir, "settings.local.json"))
	}
	before, active, err := qoderMCPLayers(bounded, paths, agent, s.Global, uninstall)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for i, path := range paths {
		if i != active {
			changes = append(changes, before[i])
			continue
		}
		plan, err := planSharedMCP(bounded, path, agent, MCPConfig{Format: ConfigJSONC, ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
		if err != nil {
			return nil, err
		}
		if err := checkPlannedOriginal(plan, before[i]); err != nil {
			return nil, err
		}
		changes = append(changes, plan...)
	}
	return changes, nil
}

func PlanQoderSkill(s Scope, uninstall bool) ([]Change, error) {
	return planQoderSkill(s, "qoder", uninstall)
}

func PlanQoderCNSkill(s Scope, uninstall bool) ([]Change, error) {
	return planQoderSkill(s, "qoder-cn", uninstall)
}

func planQoderSkill(s Scope, owner string, uninstall bool) ([]Change, error) {
	root, err := qoderDirectory(s, owner)
	if err != nil {
		return nil, err
	}
	return planSkillAsset(s, filepath.Join(root, "skills"), "qoder-skill", owner, skillContent, uninstall)
}

func init() {
	for _, name := range []string{"qoder", "qoder-cn"} {
		host, _ := hook.Lookup(name)
		caps := host.Capabilities
		caps.Skill = hook.Support{Level: hook.Supported, Detail: "User config root skills (QODER_CONFIG_DIR / QODERCN_CONFIG_DIR); shared project .qoder/skills"}
		RegisterAgent(Agent{Name: name, Method: NativeHooks, Capabilities: caps,
			PlanSkill: func(s Scope, uninstall bool) ([]Change, error) { return planQoderSkill(s, name, uninstall) },
			Plan:      func(s Scope, c Commands, uninstall bool) ([]Change, error) { return planQoder(s, c, name, uninstall) },
			PlanMCP: func(s Scope, exe string, uninstall bool) ([]Change, error) {
				return planQoderMCP(s, exe, name, uninstall)
			},
		})
	}
}

func qoderHookAdapter(agent string) hookAdapter {
	events := []HookRegistration{
		{Event: "SessionStart"}, {Event: "UserPromptSubmit"},
		{Event: "PostToolUse", Matcher: "Write|Edit|create_file|search_replace|edit_file"},
		{Event: "Stop"}, {Event: "SessionEnd"}, {Event: "PreToolUse", Optional: true},
	}
	if agent == "lingma" {
		return lingmaAdapter
	}
	return hookAdapter{
		config:  HookConfig{Agent: agent, Shape: GroupedHooks, Format: ConfigJSONC, Events: events},
		handler: nativeHookHandler(map[string]any{"type": "command", "timeout": 5}, false),
	}
}

func qoderProjectConflict(before Change) error {
	data, _ := configJSON(before.orig, ConfigJSONC)
	var root struct {
		Hooks map[string][]struct{ Hooks []json.RawMessage }
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	for _, groups := range root.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if (HookConfig{Agent: "qoder-cn"}).owns(handler) {
					return fmt.Errorf("%s contains an independent qoder-cn hook; review/remove that registration before switching to the shared qoder project hook", before.Path)
				}
			}
		}
	}
	return nil
}

func qoderMCPLayers(bounded Scope, paths []string, agent string, global, uninstall bool) ([]Change, int, error) {
	var err error
	before := make([]Change, len(paths))
	entries := make([][]byte, len(paths))
	active, definitions := 0, 0
	if !global {
		active = 1
	}
	for i, path := range paths {
		before[i], err = qoderConfig(bounded, path)
		if err != nil {
			return nil, 0, err
		}
		data, err := configJSON(before[i].orig, ConfigJSONC)
		if err != nil {
			return nil, 0, err
		}
		entries[i], err = configMember(data, []string{"mcpServers", "obituary"})
		if err != nil {
			return nil, 0, err
		}
		if entries[i] != nil {
			active, definitions = i, definitions+1
		}
	}
	if definitions > 1 {
		return nil, 0, fmt.Errorf("%s MCP obituary exists in multiple layers; review settings.json, .mcp.json and settings.local.json before installation or removal", agent)
	}
	if !global && !uninstall && entries[active] != nil {
		receipt, err := readSharedReceipt(bounded, paths[active]+".obituary-owners")
		if err != nil {
			return nil, 0, err
		}
		if !receipt.existed {
			return nil, 0, fmt.Errorf("%s already has obituary without a shared ownership receipt; another host may use it, so review its owners before adopting it", paths[active])
		}
	}
	return before, active, nil
}
