package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/agentid"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// The standalone loader (ZCode a9fe401e, adapters/src/config) uses strict JSON,
// ~/.zcode/cli/config.json and both project layers. Do not redirect this to the
// separate provider store. Shared checked readers and editors preserve
// configuration and ownership for each host's installation.
func zcodeConfigPaths(s Scope, uninstall bool) ([]string, error) {
	if s.Global {
		root, err := userdirs.ZCodeDir()
		return []string{filepath.Join(root, "cli", "config.json")}, err
	}
	if !filepath.IsAbs(s.Project) {
		return nil, fmt.Errorf("ZCode requires an absolute project root")
	}
	paths := []string{filepath.Join(s.Project, "zcode.json"), filepath.Join(s.Project, ".zcode", "config.json")}
	var existing []string
	for _, path := range paths {
		ch, err := readIntegrationChange(s, path)
		if err != nil {
			return nil, err
		}
		if ch.existed {
			existing = append(existing, path)
		}
	}
	if uninstall {
		return paths, nil
	}
	if len(existing) > 1 {
		return nil, fmt.Errorf("ZCode has both zcode.json and .zcode/config.json: consolidate the intended hook/MCP declarations into one project configuration before retrying; no layers changed")
	}
	if len(existing) == 1 {
		return existing, nil
	}
	return paths[:1], nil
}

func zcodeConfig(s Scope, path string) (Change, error) {
	ch, err := readIntegrationChange(nativeSpecialScope(s, path), path)
	if err != nil {
		return ch, err
	}
	if ch.existed && (len(bytes.TrimSpace(ch.orig)) == 0 || bytes.HasPrefix(ch.orig, []byte("\xef\xbb\xbf"))) {
		return ch, fmt.Errorf("%s: ZCode JSON.parse requires nonempty UTF-8 JSON without BOM; left untouched", path)
	}
	raw, err := configJSON(ch.orig, ConfigJSON)
	if err != nil {
		return ch, err
	}
	for _, member := range [][]string{{"hooks", "enabled"}, {"features", "mcp"}} {
		value, err := configMember(raw, member)
		if err != nil || value != nil && !sameConfigJSON(value, []byte("true")) && !sameConfigJSON(value, []byte("false")) {
			return ch, fmt.Errorf("%s: %v must be a boolean; left untouched", path, member)
		}
	}
	return ch, nil
}

// PlanZCode never grants workspace trust. Changing a declaration (including the
// executable) requires a fresh digest-bound review/grant in the host.
func PlanZCode(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("ZCode delete guard is limited: Bash can run Git Bash, CMD or legacy-shell and hook input does not identify the selection; use obit rm/bury explicitly or obit mcp until a verified shell/backend contract is available; no PreToolUse hook installed")
	}
	paths, err := zcodeConfigPaths(s, uninstall)
	if err != nil {
		return nil, err
	}
	if !uninstall && !OwnedHookExec(commands.Executable, []string{"hook", agentid.ZCode}, agentid.ZCode, nil) {
		return nil, fmt.Errorf("ZCode process hooks require a fixed absolute obit.exe path")
	}
	spec, err := zcodeAdapter.hooks(s, commands, uninstall)
	if err != nil {
		return nil, err
	}
	trustSteps := ""
	if !s.Global && !uninstall {
		workspace := "'" + strings.ReplaceAll(s.Project, "'", "'\"'\"'") + "'"
		trustSteps = fmt.Sprintf("Project hooks require trust review (Bash): zcode hooks trust review --workspace %s --json; review the commands, then zcode hooks trust grant --workspace %s --hook-digest <digest-from-review>; zcode hooks trust status --workspace %s --json; repeat after declaration/executable changes and create a new session (skipped SessionStart is not replayed)", workspace, workspace, workspace)
	}
	var changes []Change
	for _, path := range paths {
		before, err := zcodeConfig(s, path)
		if err != nil {
			return nil, err
		}
		ch, err := PlanHookConfig(nativeSpecialScope(s, path), path, spec, uninstall)
		if err != nil {
			return nil, err
		}
		if err := nativeSpecialOriginal(ch, before); err != nil {
			return nil, err
		}
		ch.Detail = "ZCode CLI process hooks; timeoutMs=5000; Stop silent; no SessionEnd or verified delete guard"
		if trustSteps != "" {
			ch.Detail += "; " + trustSteps
		}
		ch.phase = 1
		if uninstall {
			ch.phase = -1
		}
		changes = append(changes, ch)
	}
	return planZCodeRules(s, changes, trustSteps, uninstall)

}

// Keep user-specified enabled:false on refresh; the plan explicitly reports it.
// Type is required by ZCode even for an otherwise valid command/args entry.
type zcodeMCP struct{ CommandArgsMCP }

func (zcodeMCP) Encode(executable string, existing json.RawMessage) (json.RawMessage, error) {
	raw, err := (CommandArgsMCP{Type: "stdio"}).Encode(executable, existing)
	if err != nil {
		return nil, err
	}
	edits := []ConfigEdit{{Path: []string{"type"}, Value: json.RawMessage(`"stdio"`)}}
	enabled, err := configMember(raw, []string{"enabled"})
	if err != nil {
		return nil, err
	}
	if enabled == nil {
		edits = append(edits, ConfigEdit{Path: []string{"enabled"}, Value: json.RawMessage("true")})
	} else if !sameConfigJSON(enabled, []byte("true")) && !sameConfigJSON(enabled, []byte("false")) {
		return nil, fmt.Errorf("ZCode MCP enabled must be a boolean")
	}
	return editConfig(raw, ConfigJSON, edits)
}

func PlanZCodeMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	paths, err := zcodeConfigPaths(s, uninstall)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for _, path := range paths {
		before, err := zcodeConfig(s, path)
		if err != nil {
			return nil, err
		}
		bounded := nativeSpecialScope(s, path)
		ch, err := PlanMCPConfig(bounded, path, MCPConfig{ServersPath: []string{"mcp", "servers"}, Codec: zcodeMCP{}}, executable, uninstall)
		if err != nil {
			return nil, err
		}
		if err := nativeSpecialOriginal(ch, before); err != nil {
			return nil, err
		}
		if !uninstall && s.MCP {
			gate, err := PlanConfigEdits(bounded, path, ConfigJSON, []ConfigEdit{{Path: []string{"features", "mcp"}, Value: json.RawMessage("true")}})
			if err != nil {
				return nil, err
			}
			combined, err := ComposeAgentChanges([]Change{ch, gate})
			if err != nil {
				return nil, err
			}
			ch = combined[0]
		}
		if !uninstall {
			ch.Detail = "ZCode CLI stdio MCP; hooks.enabled unchanged; existing enabled:false is preserved (set mcp.servers.obituary.enabled=true to activate); user servers override same-name project servers"
			if s.MCP {
				ch.Detail += "; features.mcp enabled by explicit --mcp"
			} else {
				ch.Detail += "; features.mcp unchanged during implicit refresh; use explicit --mcp to enable it"
			}
		}
		changes = append(changes, ch)
	}
	return changes, nil
}

// docs/usage/08-skills.md documents both roots and the feature gates. Install
// only the owned skill; do not silently enable every skill in the host.
func PlanZCodeSkill(s Scope, uninstall bool) ([]Change, error) {
	root := filepath.Join(s.Project, ".zcode")
	if s.Global {
		var err error
		root, err = userdirs.ZCodeDir()
		if err != nil {
			return nil, err
		}
	} else if !filepath.IsAbs(s.Project) {
		return nil, fmt.Errorf("ZCode skills require an absolute project root")
	}
	path := filepath.Join(root, "skills", "obituary", "SKILL.md")
	changes, err := PlanManagedAssets(nativeSpecialScope(s, path), []ManagedAsset{{Path: path, ID: "zcode-obituary-skill", Owner: agentid.ZCode, Version: "1", Content: skillContent}}, uninstall)
	if err != nil {
		return nil, err
	}
	for i := range changes {
		if changes[i].Path == path && changes[i].Action != "preserve" && !uninstall {
			changes[i].Detail = "ZCode skill file only; to activate, set features.skill=true and skills.enabled=true in the intended runtime config, review per-skill overrides/name conflicts, then start a new process and run zcode skills list --cwd <project> --json to verify the obituary source"
		}
	}
	return changes, nil
}

func init() {
	host, _ := hook.Lookup(agentid.ZCode)
	RegisterAgent(Agent{Name: agentid.ZCode, Method: NativeHooks, Capabilities: host.Capabilities, Plan: PlanZCode, PlanMCP: PlanZCodeMCP, PlanSkill: PlanZCodeSkill})
}

var zcodeAdapter = hookAdapter{
	config: HookConfig{Agent: agentid.ZCode, Shape: GroupedHooks,
		ContainerPath: []string{"hooks", "events"}, EnablePath: []string{"hooks", "enabled"},
		Events: []HookRegistration{{Event: "SessionStart"}, {Event: "UserPromptSubmit"}, {Event: "PostToolUse", Matcher: "Write|Edit|ApplyPatch"}, {Event: "Stop"}}},
	removeEvents: []HookRegistration{{Event: "PreToolUse"}},
	handler: func(commands Commands, _ string) (json.RawMessage, error) {
		return marshal(map[string]any{"type": "process", "command": commands.Executable, "args": []string{"hook", agentid.ZCode}, "timeoutMs": 5000})
	},
}

func planZCodeRules(s Scope, changes []Change, trustSteps string, uninstall bool) ([]Change, error) {
	rulesPath := filepath.Join(s.Project, "AGENTS.md")
	if s.Global {
		root, err := userdirs.ZCodeDir()
		if err != nil {
			return nil, err
		}
		rulesPath = filepath.Join(root, "AGENTS.md")
	}
	rules, err := planPluginRules(nativeSpecialScope(s, rulesPath), rulesPath, agentid.ZCode, uninstall)
	if err != nil {
		return nil, err
	}
	if trustSteps != "" {
		// Config composition replaces that Change's detail. Keep the trust
		// instructions on the independent rules change as well.
		rules.Detail = trustSteps
	}
	return append(changes, rules), nil
}
