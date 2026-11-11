package integrate

import (
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func geminiDirectory(s Scope) (string, error) {
	if s.Global {
		return userdirs.GeminiDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("gemini requires an absolute project directory")
	}
	return filepath.Join(s.Project, ".gemini"), nil
}

// google-gemini/gemini-cli settings.ts parses comments, and hookRunner.ts uses
// getShellConfiguration: PowerShell on Windows. No Bash wrapper is needed.
var geminiAdapter = hookAdapter{
	directory: geminiDirectory, file: "settings.json",
	guardReason: "delete guard for Gemini is unsupported: run_shell_command does not prove its shell/backend; no BeforeTool hook was installed",
	config: HookConfig{Agent: "gemini", Shape: GroupedHooks, Format: ConfigJSONC, EventArgs: []string{},
		Events: []HookRegistration{{Event: "SessionStart"}, {Event: "AfterTool", Matcher: "write_file|replace"}, {Event: "AfterAgent"}, {Event: "SessionEnd"}}},
	removeEvents: []HookRegistration{{Event: "BeforeTool", Matcher: "run_shell_command"}},
	handler:      nativeHookHandler(map[string]any{"type": "command", "timeout": 5000}, true),
	rulesFile:    "GEMINI.md", projectRules: true,
}

func PlanGemini(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return geminiAdapter.plan(s, commands, uninstall)
}

// The CLI must combine hook and MCP edits to this same settings.json before
// Apply, or replan the second operation after applying the first. Appending two
// independently planned Changes with the same original is not a transaction.
func PlanGeminiMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	dir, err := geminiDirectory(s)
	if err != nil {
		return nil, err
	}
	return planNonemptyMCP(s, filepath.Join(dir, "settings.json"), MCPConfig{Format: ConfigJSONC, ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
}

func PlanGeminiSkill(s Scope, uninstall bool) ([]Change, error) {
	dir, err := geminiDirectory(s)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(s.Project, ".agents", "skills")
	if s.Global {
		root, err = userdirs.GeminiSkillsDir()
		if err != nil {
			return nil, err
		}
	}
	return planMigratedSharedSkill(s, root, filepath.Join(dir, "skills"), "gemini-obituary-skill", "gemini", uninstall)
}

func init() {
	host, _ := hook.Lookup("gemini")
	capabilities := host.Capabilities
	capabilities.MCP = hook.Support{Level: hook.Supported, Detail: "mcpServers in .gemini/settings.json; shares file with hooks"}
	capabilities.Skill = hook.Support{Level: hook.Supported, Detail: "Shared .agents/skills; GEMINI_CLI_HOME redirects the user home; owned .gemini copies migrate"}
	RegisterAgent(Agent{Name: "gemini", Method: NativeHooks, Capabilities: capabilities, Plan: PlanGemini, PlanMCP: PlanGeminiMCP, PlanSkill: PlanGeminiSkill})
}
