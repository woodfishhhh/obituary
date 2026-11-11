package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
)

var lingmaAdapter = hookAdapter{
	guardReason: "delete guard for Lingma is limited: run_in_terminal/Bash do not attest the shell/backend; no PreToolUse hook installed",
	config: HookConfig{Agent: "lingma", Shape: GroupedHooks, Format: ConfigJSONC,
		Events: []HookRegistration{
			{Event: "UserPromptSubmit", Optional: true},
			{Event: "PostToolUse", Matcher: "Write|Edit|create_file|search_replace"},
			{Event: "Stop"}, {Event: "PreToolUse", Optional: true},
		}},
	handler: nativeHookHandler(map[string]any{"type": "command", "timeout": 5}, false),
}

// Lingma's old IDE/plugin loader is independent of the CN CLI. Its official
// rule-template README documents .lingma/rules/*.md, but the UI owns activation
// metadata. Keep that metadata/user edits intact rather than inventing a schema.
func PlanLingma(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if err := rejectUnsupportedGuard(s, uninstall, lingmaAdapter.guardReason); err != nil {
		return nil, err
	}
	if !uninstall && !OwnedHookCommand(commands.Native, "lingma", nil) {
		return nil, fmt.Errorf("invalid Lingma hook command")
	}
	dir, err := qoderDirectory(s, "lingma")
	if err != nil {
		return nil, err
	}
	hooks, err := qoderHookLayers(s, dir, "lingma", commands.Native, uninstall)
	if err != nil {
		return nil, err
	}
	for i := range hooks {
		hooks[i].Detail = "Lingma: successful writes leave notices pending; inspect obit proposals manually; no verified model-context channel, start/end snapshot or delete guard; restart the IDE"
	}
	if s.Global {
		detail := "Lingma only documents project rules: Personal Settings > Rules > Add > obituary > Always; paste the following instructions in each intended project (no global rule path assumed):\n" + Awareness
		if uninstall {
			detail = "Personal Settings > Rules: remove only the obituary rule you manually added; retain all other rules and metadata"
		}
		return append(hooks, Change{Path: "Lingma project rules (UI)", Action: "preserve", Manual: true, Detail: detail}), nil
	}
	rules, err := PlanManagedAssets(s, []ManagedAsset{{
		Path: filepath.Join(dir, "rules", "obituary.md"), ID: "lingma-obituary-rule", Owner: "lingma", Version: "1", Content: []byte(Awareness),
	}}, uninstall)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		if rules[i].Path == filepath.Join(dir, "rules", "obituary.md") && !uninstall {
			rules[i].Detail = "Rule text installed; in Personal Settings > Rules select obituary and Always; activation metadata/real-host loading are not verified and user edits are preserved"
		}
	}
	return OrderManagedInstallation(rules, hooks, uninstall)
}

func PlanLingmaMCP(_ Scope, executable string, uninstall bool) ([]Change, error) {
	if uninstall {
		return nil, manualf("Lingma MCP location is unverified: Personal Settings > MCP Servers, edit custom JSON and remove only mcpServers.obituary after checking obit.exe and args [\"mcp\"]")
	}
	entry, err := (CommandArgsMCP{}).Encode(executable, nil)
	if err != nil {
		return nil, err
	}
	return nil, manualf("Lingma IDE/plugin MCP requires a manual step: Personal Settings > MCP Servers > Add > custom JSON; add mcpServers.obituary = %s after checking the name is unused; no physical path was assumed", entry)
}

func init() {
	host, _ := hook.Lookup("lingma")
	RegisterAgent(Agent{Name: "lingma", Method: NativeHooks, Capabilities: host.Capabilities, Plan: PlanLingma, PlanMCP: PlanLingmaMCP})
}
