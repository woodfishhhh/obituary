package integrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanTrae also discovers the Chinese user directory when an international
// installation exists, so both installed editions receive their configuration.
// The native adapters currently support SessionStart and Stop only.
func PlanTrae(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planTrae(s, commands, "trae", uninstall)
}

func PlanTraeCN(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planTrae(s, commands, "trae-cn", uninstall)
}

func planTrae(s Scope, commands Commands, agent string, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("%s delete guard is unavailable: RunCommand does not identify the execution shell", agent)
	}
	dirs, err := traeDirectories(s, agent)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for i, dir := range dirs {
		name, command := agent, commands.Native
		if i > 0 {
			name = "trae-cn"
			if !uninstall && !OwnedHookCommand(command, agent, nil) {
				return nil, fmt.Errorf("invalid %s hook command", agent)
			}
			command = strings.TrimSuffix(strings.TrimSpace(command), "hook "+agent) + "hook " + name
		}
		ch, err := planTraeHooks(s, dir, name, command, uninstall)
		if err != nil {
			return nil, err
		}
		changes = append(changes, ch)
	}
	return changes, nil
}

func traeDirectories(s Scope, agent string) ([]string, error) {
	if !s.Global {
		return []string{filepath.Join(s.Project, ".trae")}, nil
	}
	dir, err := userdirs.AgentDir(agent)
	if err != nil {
		return nil, err
	}
	dirs := []string{dir}
	if agent != "trae" {
		return dirs, nil
	}
	cn, err := userdirs.TraeCNDir()
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(cn)
	if errors.Is(err, os.ErrNotExist) {
		return dirs, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return nil, fmt.Errorf("%s must be a regular directory", cn)
	}
	return append(dirs, cn), nil
}

var traeAdapter = hookAdapter{
	config: HookConfig{Shape: GroupedHooks, Defaults: json.RawMessage(`{"version":1}`),
		Events: []HookRegistration{{Event: "SessionStart"}, {Event: "Stop"}}},
	handler: nativeHookHandler(map[string]any{"type": "command", "timeout": 5}, false),
}

func planTraeHooks(s Scope, dir, name, command string, uninstall bool) (Change, error) {
	// The hook launcher is PowerShell; this says nothing about RunCommand.
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, `"`) || strings.HasPrefix(command, "'") {
		command = "& " + command
	}
	path := filepath.Join(dir, "hooks.json")
	bounded := nativeSpecialScope(s, dir)
	before, err := readIntegrationChange(bounded, path)
	if err != nil {
		return Change{}, err
	}
	raw, err := configJSON(before.orig, ConfigJSON)
	if err != nil {
		return Change{}, err
	}
	if !uninstall {
		other := "trae"
		if name == "trae" {
			other = "trae-cn"
		}
		if err := traeVariantConflict(raw, other); err != nil {
			return Change{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	version, err := configMember(raw, []string{"version"})
	if err != nil || version != nil && !sameConfigJSON(version, []byte("1")) {
		return Change{}, fmt.Errorf("%s: supported Trae hooks version is 1", path)
	}
	spec, err := traeAdapter.hooks(s, Commands{Native: command}, uninstall)
	if err != nil {
		return Change{}, err
	}
	spec.Agent = name
	ch, err := PlanHookConfig(bounded, path, spec, uninstall)
	if err != nil {
		return Change{}, err
	}
	if err := nativeSpecialOriginal(ch, before); err != nil {
		return Change{}, err
	}
	ch.Detail = name + ": SessionStart context/snapshot and silent Stop; write observation and delete guard unavailable"
	return ch, nil
}

func traeVariantConflict(raw []byte, other string) error {
	var root struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}
	for _, event := range []string{"SessionStart", "Stop"} {
		var groups []struct{ Hooks []json.RawMessage }
		if len(root.Hooks[event]) == 0 || json.Unmarshal(root.Hooks[event], &groups) != nil {
			continue // the shared hook editor reports invalid event containers
		}
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if (HookConfig{Agent: other}).owns(handler) {
					return fmt.Errorf("%s already owns hooks here; use that variant or explicitly uninstall it before switching", other)
				}
			}
		}
	}
	return nil
}

func PlanTraeMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	if s.Global {
		entry, _ := marshal(map[string]any{"command": executable, "args": []string{"mcp"}})
		if uninstall {
			return nil, manualf("Trae global MCP requires a manual step: Settings > MCP > Raw configuration (JSON), open mcp.json and remove mcpServers.obituary only after verifying command is obit.exe and args is [\"mcp\"]")
		}
		return nil, manualf("Trae global MCP location is unverified: Settings > MCP > Raw configuration (JSON), open mcp.json and add mcpServers.obituary = %s after checking the name is unused; command paths containing spaces require host verification", entry)
	}
	if !uninstall && strings.ContainsAny(executable, " \t\r\n") {
		return nil, fmt.Errorf("documented MCP command for Trae must be space-free: choose a verified obit.exe path without spaces, enable Settings > MCP > project-level MCP, then retry")
	}
	ch, err := PlanMCPConfig(s, filepath.Join(s.Project, ".trae", "mcp.json"), MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
	ch.Detail = "Trae project MCP; requires Settings > MCP > project-level MCP enabled"
	return []Change{ch}, err
}

func init() {
	for _, name := range []string{"trae", "trae-cn"} {
		name := name
		host, _ := hook.Lookup(name)
		RegisterAgent(Agent{Name: name, Method: NativeHooks, Capabilities: host.Capabilities,
			Plan: func(s Scope, commands Commands, uninstall bool) ([]Change, error) {
				return planTrae(s, commands, name, uninstall)
			}, PlanMCP: PlanTraeMCP})
	}
}
