package integrate

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

//go:embed plugins/hermes/__init__.py
var hermesPlugin []byte

//go:embed plugins/hermes/plugin.yaml
var hermesManifest []byte

// PlanHermes installs the global Python plugin. Hermes has no verified project
// plugin/config discovery; project installations contain only AGENTS.md rules.
func PlanHermes(s Scope, commands Commands, uninstall bool) (changes []Change, err error) {
	if !s.Global {
		var err error
		s, err = boundedAgentScope(s, s.Project)
		if err != nil {
			return nil, err
		}
		if s.GuardRM && !uninstall {
			return nil, fmt.Errorf("project scope for Hermes supports rules only; install the Python hook with --global --agent hermes --guard-rm")
		}
		return planSharedAgentRules(s, filepath.Join(s.Project, "AGENTS.md"), "hermes", uninstall)
	}
	dir, err := userdirs.HermesDir()
	if err != nil {
		return nil, err
	}
	bound, err := boundedAgentScope(s, dir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "plugins", "obituary", "__init__.py")
	payload := hermesPlugin
	if !uninstall {
		payload, err = pluginPayload(bound, path, payload, commands.Executable)
		if err != nil {
			return nil, err
		}
	}
	assets, err := PlanManagedAssets(bound, []ManagedAsset{
		{Path: path, ID: "hermes-plugin", Owner: "hermes", Version: "1", Content: payload},
		{Path: filepath.Join(filepath.Dir(path), "plugin.yaml"), ID: "hermes-plugin", Owner: "hermes", Version: "1", Content: hermesManifest, Activation: true},
	}, uninstall)
	if err != nil {
		return nil, err
	}
	registration, err := planHermesEnabled(bound, filepath.Join(dir, "config.yaml"), uninstall)
	if err != nil {
		return nil, err
	}
	if !uninstall {
		registration.Detail = "Hermes loads the Python plugin in its own runtime; only a verified local LocalEnvironment Bash backend is observed; first-use, remote, sandbox and background tools are skipped; restart Hermes to load the plugin"
	}
	config, err := planHermesConfigParents(bound, registration, "hooks", uninstall)
	return append(assets, config...), err
}

func planHermesEnabled(s Scope, path string, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	data, err := configJSON(ch.orig, ConfigYAML)
	if err != nil {
		return ch, err
	}
	raw, err := configMember(data, []string{"plugins", "enabled"})
	if err != nil {
		return ch, err
	}
	enabled := []string{}
	if raw != nil && (string(raw) == "null" || json.Unmarshal(raw, &enabled) != nil) {
		return ch, fmt.Errorf("%s: plugins.enabled must be an array of plugin names", path)
	}
	if slices.Contains(enabled, "obituary") {
		manifest := filepath.Join(filepath.Dir(path), "plugins", "obituary", "plugin.yaml")
		receipt, err := readAssetReceipt(s, manifest)
		if err != nil {
			return ch, err
		}
		state, valid := decodeAssetReceipt(receipt.orig, ManagedAsset{Path: manifest, ID: "hermes-plugin"})
		if !valid || !slices.Contains(state.Owners, "hermes") {
			return ch, fmt.Errorf("%s: obituary plugin enablement has no verified receipt; left untouched", path)
		}
		if uninstall {
			enabled = slices.DeleteFunc(enabled, func(name string) bool { return name == "obituary" })
		}
	} else if uninstall {
		return ch, nil
	} else {
		enabled = append(enabled, "obituary")
	}
	value, _ := marshal(enabled)
	return PlanConfigEdits(s, path, ConfigYAML, []ConfigEdit{{Path: []string{"plugins", "enabled"}, Value: value, CheckExpected: true, Expected: raw}})
}

func PlanHermesMCP(s Scope, executable string, uninstall bool) (changes []Change, err error) {
	if !s.Global {
		return nil, manualf("Hermes has no verified project MCP discovery; use obit init --global --agent hermes --mcp for config.yaml mcp_servers.obituary (add --uninstall to remove the global registration)")
	}
	dir, err := userdirs.HermesDir()
	if err != nil {
		return nil, err
	}
	bound, err := boundedAgentScope(s, dir)
	if err != nil {
		return nil, err
	}
	ch, err := PlanMCPConfig(bound, filepath.Join(dir, "config.yaml"), MCPConfig{Format: ConfigYAML, ServersPath: []string{"mcp_servers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
	if err != nil {
		return nil, err
	}
	return planHermesConfigParents(bound, ch, "mcp", uninstall)
}

func PlanHermesSkill(s Scope, uninstall bool) ([]Change, error) {
	if !s.Global {
		return PlanSharedSkill(s, "hermes", uninstall)
	}
	root, err := userdirs.HermesDir()
	if err != nil {
		return nil, err
	}
	return planSkillAsset(s, filepath.Join(root, "skills"), "hermes-skill", "hermes", skillContent, uninstall)
}

func init() {
	host, _ := hook.Lookup("hermes")
	caps := host.Capabilities
	caps.Skill = hook.Support{Level: hook.Limited, Detail: "HERMES_HOME/skills (Windows default LOCALAPPDATA/hermes/skills); shared project .agents/skills requires project trust"}
	RegisterAgent(Agent{Name: "hermes", Method: Plugin, Capabilities: caps, Plan: PlanHermes, PlanMCP: PlanHermesMCP, PlanSkill: PlanHermesSkill})
}
