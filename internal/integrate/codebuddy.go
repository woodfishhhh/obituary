package integrate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func codeBuddyDirectory(s Scope, owner string) (string, error) {
	if s.Global {
		if owner == "codebuddy-ide" {
			return userdirs.CodeBuddyIDEDir()
		}
		return userdirs.CodeBuddyDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("CodeBuddy requires an absolute project directory")
	}
	return filepath.Join(s.Project, ".codebuddy"), nil
}

func codeBuddyConfig(s Scope, path string) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err == nil {
		err = nonemptyConfig(ch)
	}
	return ch, err
}

func codeBuddyHasHooks(raw []byte, agents ...string) (bool, error) {
	var config struct {
		Hooks map[string][]struct{ Hooks []json.RawMessage } `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return false, err
	}
	for _, groups := range config.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				for _, name := range agents {
					if (HookConfig{Agent: name}).owns(handler) {
						return true, nil
					}
				}
			}
		}
	}
	return false, nil
}

// CodeBuddy CLI reference merges scope hooks; IDE reference replaces them.
// Refuse a second CLI registration instead of silently executing twice. The
// project plan includes the observed global file, so a concurrent edit fails.
func codeBuddyGlobalCheck(s Scope, owner, dir string, uninstall bool) ([]Change, error) {
	if s.Global || uninstall || owner != "codebuddy" {
		return nil, nil
	}
	global, err := userdirs.CodeBuddyDir()
	if err != nil || strings.EqualFold(global, dir) {
		return nil, err
	}
	ch, err := codeBuddyConfig(nativeSpecialScope(Scope{Global: true}, global), filepath.Join(global, "settings.json"))
	if err != nil {
		return nil, err
	}
	raw, err := configJSON(ch.orig, ConfigJSONC)
	if err != nil {
		return nil, err
	}
	if present, err := codeBuddyHasHooks(raw, "codebuddy", "codebuddy-ide"); err != nil {
		return nil, err
	} else if present {
		return nil, fmt.Errorf("CodeBuddy CLI merges user and project hooks: %s already registers Obituary; keep the user installation or uninstall it before installing project hooks", ch.Path)
	}
	return []Change{ch}, nil
}

// PlanCodeBuddy implements native grouped hooks with the shared JSONC editor
// so CLI/IDE registration preserves existing configuration and ownership checks.
func PlanCodeBuddy(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planCodeBuddy(s, commands, "codebuddy", uninstall)
}

func PlanCodeBuddyIDE(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return planCodeBuddy(s, commands, "codebuddy-ide", uninstall)
}

func planCodeBuddy(s Scope, commands Commands, owner string, uninstall bool) ([]Change, error) {
	dir, err := codeBuddyDirectory(s, owner)
	if err != nil {
		return nil, err
	}
	checks, err := codeBuddyGlobalCheck(s, owner, dir, uninstall)
	if err != nil {
		return nil, err
	}
	bounded := nativeSpecialScope(s, dir)
	path := filepath.Join(dir, "settings.json")
	before, err := codeBuddyConfig(bounded, path)
	if err != nil {
		return nil, err
	}
	rulesPath := filepath.Join(s.Project, "CODEBUDDY.md")
	if s.Global {
		rulesPath = filepath.Join(dir, "CODEBUDDY.md")
	}
	raw, err := configJSON(before.orig, ConfigJSONC)
	if err != nil {
		return nil, err
	}
	command, err := codeBuddyHookCommand(commands, raw, path, uninstall)
	if err != nil {
		return nil, err
	}
	spec, err := codeBuddyAdapter.hooks(s, Commands{Native: command}, uninstall)
	if err != nil {
		return nil, err
	}
	hooks, err := PlanHookConfig(bounded, path, spec, uninstall)
	if err != nil {
		return nil, err
	}
	if err := nativeSpecialOriginal(hooks, before); err != nil {
		return nil, err
	}
	rules, err := nativeSpecialBlock(bounded, rulesPath, Awareness, uninstall)
	if err != nil {
		return nil, err
	}
	hooks.Detail = "CodeBuddy CLI/IDE shared hooks; CLI requires Git Bash and /hooks review; IDE project settings override user settings; delete guard requires the host's agent_shells declaration (bash or powershell); undeclared commands pass through"
	asset := ManagedAsset{Path: filepath.Join(dir, ".obituary-hooks"), ID: "codebuddy-hooks", Owner: owner, Version: "1", Content: []byte("Obituary shared registration: codebuddy-hooks\n")}
	return planSharedRegistration(bounded, asset, append(checks, hooks, rules), []string{owner}, uninstall)
}

// PlanCodeBuddyMCP preserves the documented first-existing-file priority.
// Legacy .codebuddy.json is runtime state and always requires the host CLI/UI.
func PlanCodeBuddyMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	dir, err := codeBuddyDirectory(s, "codebuddy")
	if err != nil {
		return nil, err
	}
	paths := []string{filepath.Join(s.Project, ".mcp.json"), filepath.Join(s.Project, "mcp.json")}
	legacy := ""
	if s.Global {
		legacy, err = userdirs.CodeBuddyLegacyMCP()
		if err != nil {
			return nil, err
		}
		paths = []string{filepath.Join(dir, ".mcp.json"), filepath.Join(dir, "mcp.json"), legacy}
	}
	selected := paths[0]
	var checks []Change
	for _, path := range paths {
		ch, err := codeBuddyConfig(nativeSpecialScope(s, path), path)
		if err != nil {
			return nil, err
		}
		if ch.existed {
			selected = path
			break
		}
		checks = append(checks, ch)
	}
	if selected == legacy {
		detail := fmt.Sprintf("Legacy CodeBuddy runtime state is not edited: inspect the obituary server with /mcp; if the name is unused, run codebuddy mcp add --scope user obituary -- %s mcp; retain existing server options when refreshing, and do not create a higher-priority .mcp.json that hides existing services", strconv.Quote(filepath.ToSlash(executable)))
		if uninstall {
			detail = "Legacy CodeBuddy runtime state is not edited: in /mcp remove only the user-scoped obituary server after verifying command is obit.exe and args is [mcp]"
		}
		return []Change{{Path: legacy, Action: "preserve", Manual: true, Detail: detail}}, nil
	}
	plan, err := PlanCodeBuddyMCPFile(s, selected, "codebuddy", executable, uninstall)
	if err != nil {
		return nil, err
	}
	// The selected file already has its original-content check in plan.
	checks = slices.DeleteFunc(checks, func(c Change) bool { return c.Path == selected })
	return append(checks, plan...), nil
}

// PlanCodeBuddyMCPFile is the explicit-file entry point for IDE/plugin UIs.
// The caller selects a verified mcpServers JSON/JSONC file, never a guessed
// editor profile location. WorkBuddy may reuse it only for explicit targets.
func PlanCodeBuddyMCPFile(s Scope, path, owner, executable string, uninstall bool) ([]Change, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("CodeBuddy MCP needs an explicit absolute configuration file")
	}
	s = nativeSpecialScope(s, path)
	before, err := codeBuddyConfig(s, path)
	if err != nil {
		return nil, err
	}
	changes, err := planSharedMCP(s, path, owner, MCPConfig{Format: ConfigJSONC, ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
	if err != nil {
		return nil, err
	}
	for _, ch := range changes {
		if ch.Path == path {
			if err := nativeSpecialOriginal(ch, before); err != nil {
				return nil, err
			}
		}
	}
	return changes, nil
}

func PlanCodeBuddyIDEMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	entry, _ := json.Marshal(map[string]any{"command": executable, "args": []string{"mcp"}})
	if uninstall {
		return nil, manualf("CodeBuddy IDE/plugin MCP: open Settings > MCP > configuration, verify the obituary command is obit.exe with args [mcp], then remove only mcpServers.obituary; the global/profile file location is not verified")
	}
	return nil, manualf("CodeBuddy IDE/plugin MCP: open Settings > MCP > configuration (plugin: MCP Server > edit settings.json), then add mcpServers.obituary = %s after checking the name is unused; automatic profile discovery is unavailable", entry)
}

// PlanCodeBuddySharedSkill exposes the documented .codebuddy compatibility
// root with separate owners for CodeBuddy, IDE and an explicit WorkBuddy opt-in.
func PlanCodeBuddySharedSkill(s Scope, owner string, uninstall bool) ([]Change, error) {
	dir, err := codeBuddyDirectory(s, owner)
	if err != nil {
		return nil, err
	}
	return PlanManagedAssets(nativeSpecialScope(s, dir), []ManagedAsset{{Path: filepath.Join(dir, "skills", "obituary", "SKILL.md"), ID: "codebuddy-skill", Owner: owner, Version: "1", Content: skillContent}}, uninstall)
}

func init() {
	for _, owner := range []string{"codebuddy", "codebuddy-ide"} {
		host, _ := hook.Lookup(owner)
		caps := host.Capabilities
		caps.MCP = hook.Support{Level: hook.Limited, Detail: "CLI first-existing JSONC MCP; legacy runtime state and IDE/plugin profile targets require manual selection"}
		caps.Skill = hook.Support{Level: hook.Limited, Detail: ".codebuddy/skills with shared owner receipts; host trust/enablement remains manual"}
		mcp := PlanCodeBuddyMCP
		if owner == "codebuddy-ide" {
			mcp = PlanCodeBuddyIDEMCP
		}
		RegisterAgent(Agent{Name: owner, Method: NativeHooks, Capabilities: caps,
			Plan: func(s Scope, c Commands, uninstall bool) ([]Change, error) {
				return planCodeBuddy(s, c, owner, uninstall)
			},
			PlanMCP: mcp, PlanSkill: func(s Scope, uninstall bool) ([]Change, error) { return PlanCodeBuddySharedSkill(s, owner, uninstall) },
		})
	}
}

var codeBuddyAdapter = hookAdapter{
	config: HookConfig{Agent: "codebuddy", Shape: GroupedHooks, Format: ConfigJSONC,
		Events: []HookRegistration{
			{Event: "SessionStart"}, {Event: "PostToolUse", Matcher: "Write|Edit|MultiEdit"},
			{Event: "UserPromptSubmit"}, {Event: "Stop"}, {Event: "SessionEnd"}, {Event: "PreToolUse"},
		}},
	guardEvent: "PreToolUse",
	handler:    nativeHookHandler(map[string]any{"type": "command", "timeout": 5}, false),
}

func codeBuddyHookCommand(commands Commands, raw []byte, path string, uninstall bool) (string, error) {
	// Use a literal Windows path with forward slashes: Git Bash accepts it,
	// unlike the /mnt/c path produced for WSL. OwnedHookCommand rejects any
	// interpolation or compound command; no new shell tokenizer is needed.
	command := strconv.Quote(filepath.ToSlash(commands.Executable)) + " hook codebuddy"
	if !uninstall && (!filepath.IsAbs(commands.Executable) || !OwnedHookCommand(command, "codebuddy", nil)) {
		return "", fmt.Errorf("CodeBuddy hooks need a literal absolute obit.exe path")
	}
	if !uninstall {
		if present, err := codeBuddyHasHooks(raw, "codebuddy-ide"); err != nil {
			return "", err
		} else if present {
			return "", fmt.Errorf("%s: remove the separate codebuddy-ide registration before installing shared hooks", path)
		}
	}
	return command, nil
}
