package integrate

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

//go:embed plugins/kilocode/obituary.ts
var kiloCodePlugin []byte

func kiloCodeRoot(s Scope) (string, Scope, error) {
	if !s.Global {
		bound, err := boundedAgentScope(s, s.Project)
		return filepath.Join(s.Project, ".kilo"), bound, err
	}
	dir, err := userdirs.KiloCodeDir()
	if err != nil {
		return "", Scope{}, err
	}
	bound, err := boundedAgentScope(s, dir)
	return dir, bound, err
}

// PlanKiloCode targets the current @kilocode/plugin module API. Legacy Kilo
// continues to read .kilocode/rules; it cannot load this plugin or MCP schema.
func PlanKiloCode(s Scope, commands Commands, uninstall bool) (changes []Change, err error) {
	dir, bound, err := kiloCodeRoot(s)
	if err != nil {
		return nil, err
	}
	rulePath := filepath.Join(s.Project, ".kilocode", "rules", "obituary.md")
	if s.Global {
		rulePath = filepath.Join(dir, "rules", "obituary.md")
	}
	rule, err := planAgentRule(bound, rulePath, uninstall)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "plugin", "obituary.ts")
	payload := kiloCodePlugin
	if !uninstall {
		payload, err = pluginPayload(bound, path, payload, commands.Executable)
		if err != nil {
			return nil, err
		}
	}
	assets, err := PlanManagedAssets(bound, []ManagedAsset{{Path: path, ID: "kilocode-plugin", Owner: "kilocode", Version: "1", Content: payload}}, uninstall)
	for i := range assets {
		if assets[i].Path == path {
			assets[i].Detail = "current Kilo PluginModule; guard/local observation require OBIT_KILOCODE_LOCAL_SHELL=bash or powershell after verifying the executor; legacy Kilo reads project rules only"
		}
	}
	if err != nil {
		return nil, err
	}
	if s.Global {
		config, err := kiloCodeConfigPath(s, dir, bound)
		if err != nil {
			return nil, err
		}
		registration, err := planKiloCodeInstructions(bound, config, rulePath, uninstall)
		if err != nil {
			return nil, err
		}
		return OrderManagedInstallation(append(assets, rule), []Change{registration}, uninstall)
	}
	return append(assets, rule), nil
}

func kiloCodeConfigPath(s Scope, dir string, bound Scope) (string, error) {
	// Ambiguous layered configuration requires an explicit user decision.
	// Never create a second file that could shadow an existing obituary server.
	candidates := []string{filepath.Join(dir, "kilo.json"), filepath.Join(dir, "kilo.jsonc")}
	if !s.Global {
		candidates = append(candidates, filepath.Join(s.Project, "kilo.json"), filepath.Join(s.Project, "kilo.jsonc"))
	}
	selected := ""
	for _, path := range candidates {
		if err := bound.checkInside(bound.Project, path, path+".bak"); err != nil {
			return "", err
		}
		_, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if selected != "" {
			return "", fmt.Errorf("multiple Kilo configurations (%s, %s); consolidate Obituary entries in the active kilo.json[c] before installing", selected, path)
		}
		selected = path
	}
	if selected == "" {
		selected = filepath.Join(dir, "kilo.json")
	}
	return selected, nil
}

func planKiloCodeInstructions(s Scope, config, rule string, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, config)
	if err != nil {
		return ch, err
	}
	data, err := configJSON(ch.orig, ConfigJSONC)
	if err != nil {
		return ch, err
	}
	raw, err := configMember(data, []string{"instructions"})
	if err != nil {
		return ch, err
	}
	paths := []string{}
	if raw != nil && (string(raw) == "null" || json.Unmarshal(raw, &paths) != nil) {
		return ch, fmt.Errorf("%s: instructions must be an array of paths", config)
	}
	ref := filepath.ToSlash(rule)
	if slices.Contains(paths, ref) {
		owned, err := readIntegrationChange(s, rule)
		if err != nil {
			return ch, err
		}
		if !bytes.Contains(owned.orig, []byte(blockBegin)) {
			return ch, fmt.Errorf("%s: instruction reference has no owned rules block; left untouched", config)
		}
		if uninstall {
			paths = slices.DeleteFunc(paths, func(path string) bool { return path == ref })
		}
	} else if uninstall {
		return ch, nil
	} else {
		paths = append(paths, ref)
	}
	value, _ := marshal(paths)
	return PlanConfigEdits(s, config, ConfigJSONC, []ConfigEdit{{Path: []string{"instructions"}, Value: value, Expected: raw, CheckExpected: true}})
}

func PlanKiloCodeMCP(s Scope, executable string, uninstall bool) (changes []Change, err error) {
	dir, bound, err := kiloCodeRoot(s)
	if err != nil {
		return nil, err
	}
	selected, err := kiloCodeConfigPath(s, dir, bound)
	if err != nil {
		return nil, err
	}
	ch, err := PlanMCPConfig(bound, selected, MCPConfig{Format: ConfigJSONC, ServersPath: []string{"mcp"}, Codec: CommandArrayMCP{}}, executable, uninstall)
	return []Change{ch}, err
}

func PlanKiloCodeSkill(s Scope, uninstall bool) ([]Change, error) {
	return PlanSharedSkill(s, "kilocode", uninstall)
}

func init() {
	host, _ := hook.Lookup("kilocode")
	caps := host.Capabilities
	caps.Skill = hook.Support{Level: hook.Supported, Detail: "Shared .agents/skills at user and project scope"}
	RegisterAgent(Agent{Name: "kilocode", Method: Plugin, Capabilities: caps, Plan: PlanKiloCode, PlanMCP: PlanKiloCodeMCP, PlanSkill: PlanKiloCodeSkill})
}
