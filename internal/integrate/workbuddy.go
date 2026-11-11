package integrate

import (
	"embed"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// The mainland MCP guide verifies command/args and both .workbuddy scopes.
// Shared MCP planning and managed-asset receipts preserve user configuration
// and keep repeated installation idempotent.
// Consumer hook and plugin loader contracts remain unverified.
//
//go:embed plugins/workbuddy/SETUP.md plugins/workbuddy/sources.json
var workBuddyFiles embed.FS

const workBuddyManualSkill = "export only; in mainland WorkBuddy open 技能 > 添加技能 > 上传技能, review the local package, then confirm it under 已安装; upload format and loader remain unverified; no automatic activation"

func workBuddyDirectory(s Scope) (string, error) {
	if s.Global {
		return userdirs.WorkBuddyDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("WorkBuddy requires an absolute project directory")
	}
	return filepath.Join(s.Project, ".workbuddy"), nil
}

// PlanWorkBuddy exports office setup instructions. The export directory is
// Obituary-owned staging, with no host-discovered manifest or hook registration.
func PlanWorkBuddy(s Scope, _ Commands, uninstall bool) ([]Change, error) {
	if s.GuardRM && !uninstall {
		return nil, fmt.Errorf("WorkBuddy delete guard is unsupported: consumer hook registration, deny and local execution contracts are unverified; use --mcp for explicit local operations and review 插件 > MCP 服务器 > 配置 MCP")
	}
	root, err := workBuddyDirectory(s)
	if err != nil {
		return nil, err
	}
	var assets []ManagedAsset
	for _, name := range []string{"SETUP.md", "sources.json"} {
		data, err := workBuddyFiles.ReadFile("plugins/workbuddy/" + name)
		if err != nil {
			return nil, err
		}
		assets = append(assets, ManagedAsset{Path: filepath.Join(root, "obituary-export", name), ID: "workbuddy-export-" + name,
			Owner: "workbuddy", Version: "1", Content: data})
	}
	changes, err := PlanManagedAssets(nativeSpecialScope(s, root), assets, uninstall)
	for i := range changes {
		if changes[i].Action != "preserve" {
			changes[i].Detail = "mainland Windows WorkBuddy setup export only; hooks and automatic plugin activation unavailable; international MCP paths require separate UI verification; cloud/remote paths are outside local coverage"
		}
	}
	return changes, err
}

func PlanWorkBuddyMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	if !uninstall && !filepath.IsAbs(executable) {
		return nil, fmt.Errorf("WorkBuddy MCP requires an absolute local obit.exe path")
	}
	root, err := workBuddyDirectory(s)
	if err != nil {
		return nil, err
	}
	change, err := PlanMCPConfig(nativeSpecialScope(s, root), filepath.Join(root, "mcp.json"), MCPConfig{
		Format: ConfigJSON, ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{},
	}, executable, uninstall)
	if err != nil {
		return nil, err
	}
	if err := nonemptyConfig(change); err != nil {
		return nil, err
	}
	if change.Action != "preserve" {
		change.Detail = "mainland Windows WorkBuddy local MCP; review 插件 > MCP 服务器 > 配置 MCP and confirm service status; actual client unverified; keep this local server disabled for cloud/remote tasks"
	}
	return []Change{change}, nil
}

// PlanWorkBuddySkill reuses the existing human-authored bytes, outside every
// auto-discovered skill/plugin root. The user controls any later UI import.
func PlanWorkBuddySkill(s Scope, uninstall bool) ([]Change, error) {
	root, err := workBuddyDirectory(s)
	if err != nil {
		return nil, err
	}
	changes, err := PlanManagedAssets(nativeSpecialScope(s, root), []ManagedAsset{{
		Path: filepath.Join(root, "obituary-export", "SKILL.md"), ID: "workbuddy-export-skill",
		Owner: "workbuddy", Version: "1", Content: skillContent,
	}}, uninstall)
	for i := range changes {
		if changes[i].Action != "preserve" {
			changes[i].Detail = workBuddyManualSkill
			if uninstall {
				changes[i].Detail = "remove owned export only; any WorkBuddy-imported copy requires 技能 > 已安装 > … > 卸载; other files and directories are retained"
			}
		}
	}
	return changes, err
}

func init() {
	unverified := hook.Support{Detail: "unsupported: mainland consumer WorkBuddy hook schema, registration path and local/cloud identity unverified; no callbacks registered"}
	RegisterAgent(Agent{Name: "workbuddy", Method: RulesAndMCP, Plan: PlanWorkBuddy, PlanMCP: PlanWorkBuddyMCP, PlanSkill: PlanWorkBuddySkill,
		Capabilities: hook.Capabilities{
			DeleteGuard: unverified, WriteObservation: unverified, SessionStart: unverified, Stop: unverified, SessionEnd: unverified,
			Context: hook.Support{Detail: "office setup and optional SKILL.md exported for manual review; automatic rule/context loading unverified"},
			MCP:     hook.Support{Level: hook.Supported, Detail: "mainland Windows desktop: user/project .workbuddy/mcp.json, local command/args; actual client unverified; international .workbuddy-ai MCP paths unverified, verify in that edition's MCP UI; cloud/remote tasks excluded"},
			Skill:   hook.Support{Level: hook.Limited, Detail: workBuddyManualSkill},
		},
	})
}
