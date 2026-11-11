package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanOpenClaw stages a native Gateway plugin. JSON5 activation remains an
// explicit manual step; a preserve change makes Apply report incomplete setup.
func PlanOpenClaw(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	dir := filepath.Join(s.Project, ".openclaw")
	if s.Global {
		var err error
		dir, err = userdirs.OpenClawDir()
		if err != nil {
			return nil, err
		}
	} else if !filepath.IsAbs(s.Project) {
		return nil, fmt.Errorf("OpenClaw installation needs an absolute workspace")
	}
	config, err := userdirs.OpenClawConfigFile()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(dir, "extensions", "obituary")
	manual := Change{Path: config, Action: "preserve", Manual: true, Detail: fmt.Sprintf("manual activation required for native local Windows Gateway: in the OpenClaw Control UI configuration editor set plugins.entries.obituary.enabled=true; add obituary to plugins.allow if an allowlist exists; confirm discovery of %s with openclaw plugins list, then reload the Gateway yourself; JSON5 is left unchanged; WSL/remote Gateway requires its own executable and environment bridge", root)}
	if uninstall {
		manual.Detail = fmt.Sprintf("manual unregistration required: in the OpenClaw Control UI for %s disable plugins.entries.obituary, remove obituary from plugins.allow and remove any %s entry from plugins.load.paths, then reload the Gateway yourself; JSON5 activation cannot be verified automatically, so plugin files and receipts under %s are retained; after verifying disabled state they may be removed manually", config, root, root)
		return []Change{manual}, nil
	}
	assets, err := planPlugin(s, dir, "openclaw", "openclaw-plugin", "plugins/openclaw/index.ts", filepath.Join("extensions", "obituary", "index.ts"), commands.Executable, false)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"package.json", "openclaw.plugin.json"} {
		payload, err := pluginFiles.ReadFile("plugins/openclaw/" + name)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(root, name)
		if err := s.checkInside(dir, path, ManagedAssetReceiptPath(path)); err != nil {
			return nil, err
		}
		bundle, err := PlanManagedAssets(s, []ManagedAsset{{Path: path, ID: "openclaw-" + name, Owner: "openclaw", Version: "1", Content: payload, Activation: true}}, false)
		if err != nil {
			return nil, err
		}
		assets = append(assets, bundle...)
	}
	if !s.Global {
		block, err := planPluginRules(s, filepath.Join(s.Project, "AGENTS.md"), "openclaw", false)
		if err != nil {
			return nil, err
		}
		assets = append(assets, block)
	} else {
		manual.Detail += "; global workspace rules have no unique target: add Obituary instructions to the active agent workspace AGENTS.md manually"
	}
	return append(assets, manual), nil
}

func PlanOpenClawMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	config, err := userdirs.OpenClawConfigFile()
	if err != nil {
		return nil, err
	}
	operation := "remove mcp.servers.obituary only after verifying command is obit.exe and args is [\"mcp\"]"
	if !uninstall {
		entry, err := (CommandArgsMCP{}).Encode(executable, nil)
		if err != nil {
			return nil, err
		}
		operation = "set mcp.servers.obituary to " + string(entry) + " only if absent or owned by Obituary"
	}
	return []Change{{Path: config, Action: "preserve", Manual: true, Detail: "manual MCP step required: in the OpenClaw Control UI for " + config + ", " + operation + "; Gateway owns this JSON5 state; confirm native local Windows execution, then reload it yourself; project MCP activation also uses this Gateway configuration"}}, nil
}

func PlanOpenClawSkill(s Scope, uninstall bool) (changes []Change, err error) {
	defer func() {
		if uninstall && err == nil {
			changes, err = retainOpenClawSkill(s, changes)
		}
	}()
	if !s.Global {
		return PlanSharedSkill(s, "openclaw", uninstall)
	}
	root, shared, err := userdirs.OpenClawSkillsDir()
	if err != nil {
		return nil, err
	}
	id := "openclaw-skill"
	if shared {
		id = sharedSkillID
	}
	return planSkillAsset(s, root, id, "openclaw", skillContent, uninstall)
}

func retainOpenClawSkill(s Scope, changes []Change) ([]Change, error) {
	dir := filepath.Join(s.Project, ".openclaw")
	if s.Global {
		var err error
		dir, err = userdirs.OpenClawDir()
		if err != nil {
			return nil, err
		}
	}
	manifest, err := readIntegrationChange(s, filepath.Join(dir, "extensions", "obituary", "openclaw.plugin.json"))
	if err != nil {
		return nil, err
	}
	if !manifest.existed {
		return changes, nil
	}
	for i := range changes {
		ch := &changes[i]
		ch.Action, ch.data, ch.delete, ch.next = "unchanged", nil, false, nil
		ch.asset, ch.shared, ch.relocated = nil, nil, nil
		if ch.existed {
			ch.Action = "preserve"
			ch.Detail = "OpenClaw skill and owner receipt retained until manual Gateway deactivation is verified; remove them only after disabling plugins.entries.obituary and reloading the Gateway"
		}
	}
	return append(changes, manifest), nil
}

func init() {
	host, _ := hook.Lookup("openclaw")
	caps := host.Capabilities
	caps.Skill = hook.Support{Level: hook.Supported, Detail: "Shared workspace .agents/skills; default-state user .agents/skills; isolated state uses OPENCLAW_STATE_DIR/skills"}
	caps.DeleteGuard.Detail = "before_tool_call block/blockReason; manual Gateway activation plus OBIT_OPENCLAW_LOCAL=1 and OBIT_OPENCLAW_SHELL required; Code Mode skipped"
	caps.Stop.Detail = "agent_end maps to silent Stop after local backend attestation"
	caps.SessionEnd.Detail = "session_end maps to silent SessionEnd with a 1.2-second subprocess budget"
	caps.MCP.Detail = "Manual Control UI mcp.servers entry; JSON5 and Gateway ownership prevent automatic edits"
	RegisterAgent(Agent{Name: "openclaw", Method: Plugin, Capabilities: caps, Plan: PlanOpenClaw, PlanMCP: PlanOpenClawMCP, PlanSkill: PlanOpenClawSkill})
}
