package integrate

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Cursor's documented hooks/MCP/skills locations; see cursor.com/docs/{hooks,
// rules,mcp,skills}.
func cursorDirectory(s Scope) (string, error) {
	if s.Global {
		return userdirs.CursorDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("cursor requires an absolute project directory")
	}
	return filepath.Join(s.Project, ".cursor"), nil
}

var cursorAdapter = hookAdapter{
	directory: cursorDirectory, file: "hooks.json",
	guardReason: "delete guard for Cursor is unsupported: hooks do not identify the actual shell/backend; no pre hook was installed",
	config: HookConfig{Agent: "cursor", Shape: FlatHooks, EventArgs: []string{}, Defaults: json.RawMessage(`{"version":1}`),
		Events: []HookRegistration{{Event: "sessionStart"}, {Event: "stop"}, {Event: "sessionEnd"}}},
	removeEvents: []HookRegistration{{Event: "preToolUse"}, {Event: "beforeShellExecution"}, {Event: "postToolUse"}, {Event: "afterFileEdit"}},
	handler:      nativeHookHandler(map[string]any{"timeout": 5, "failClosed": false}, true),
	check:        func(ch Change) error { return hookVersion(ch, "Cursor") },
	finish:       planCursorRules,
}

func PlanCursor(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return cursorAdapter.plan(s, commands, uninstall)
}

func planCursorRules(s Scope, dir string, hooks Change, uninstall bool) ([]Change, error) {
	if s.Global {
		detail := "Manual step in Cursor Customize > Rules > User Rules: append this block without replacing other rules:\n" + blockBegin + "\n" + Awareness + blockEnd
		if uninstall {
			detail = "Manual step in Cursor Customize > Rules > User Rules: if previously added, remove only the Obituary block between " + blockBegin + " and " + blockEnd + "; keep all other rules"
		}
		return []Change{hooks, {Path: "Cursor User Rules (UI)", Action: "preserve", Manual: true, Detail: detail}}, nil
	}
	rules, err := PlanManagedAssets(s, []ManagedAsset{{Path: filepath.Join(dir, "rules", "obituary.mdc"), ID: "cursor-obituary-rule", Owner: "cursor", Version: "1",
		Content: []byte("---\ndescription: Obituary, the trash bin for AI agents — restorable deletion and file lifespans\nalwaysApply: true\n---\n\n" + Awareness)}}, uninstall)
	if err != nil {
		return nil, err
	}
	return OrderManagedInstallation(rules, []Change{hooks}, uninstall)
}

func PlanCursorMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	dir, err := cursorDirectory(s)
	if err != nil {
		return nil, err
	}
	return planNonemptyMCP(s, filepath.Join(dir, "mcp.json"), MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
}

func PlanCursorSkill(s Scope, uninstall bool) ([]Change, error) {
	dir, err := cursorDirectory(s)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(s.Project, ".agents", "skills")
	if s.Global {
		root, err = userdirs.AgentSkillsDir()
		if err != nil {
			return nil, err
		}
	}
	return planMigratedSharedSkill(s, root, filepath.Join(dir, "skills"), "cursor-obituary-skill", "cursor", uninstall)
}
func init() {
	host, _ := hook.Lookup("cursor")
	capabilities := host.Capabilities
	capabilities.MCP = hook.Support{Level: hook.Supported, Detail: ".cursor/mcp.json; local stdio"}
	capabilities.Skill = hook.Support{Level: hook.Supported, Detail: "Shared .agents/skills at user and project scope; owned previous host-specific copies migrate"}
	RegisterAgent(Agent{Name: "cursor", Method: NativeHooks, Capabilities: capabilities, Plan: PlanCursor, PlanMCP: PlanCursorMCP, PlanSkill: PlanCursorSkill})
}
