package integrate

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Templates follow deepseek-ai/deepseek-harness fcc0ad46, docs/user/develop/
// basic/publish.md. Independent owned bundles preserve dynamic !!js and provider
// rows byte-for-byte; the shared YAML object editor cannot edit Cordis lists.
//
//go:embed plugins/deepseek/hooks.json plugins/deepseek/cordis.patch.yml plugins/deepseek/package.json plugins/deepseek/index.ts
var deepseekFiles embed.FS

func deepseekRoot(s Scope) (string, error) {
	if s.Global {
		return userdirs.DeepSeekDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("DeepSeek installation needs an absolute project directory")
	}
	return filepath.Join(s.Project, ".dsh"), nil
}

// PlanDeepSeek exports a bundle. Activation is deliberately explicit: no
// package manager, profile manifest, or home/profile YAML is changed by Plan.
func PlanDeepSeek(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	root, err := deepseekRoot(s)
	if err != nil {
		return nil, err
	}
	return planDeepSeek(s, commands, root, uninstall)
}

// PlanDeepSeekProfile exports a separate bundle below one explicitly selected
// profile. Home patches still take precedence over a profile's bundle layer.
func PlanDeepSeekProfile(s Scope, commands Commands, profile string, uninstall bool) ([]Change, error) {
	if !s.Global {
		return nil, fmt.Errorf("DeepSeek profile installation requires global scope")
	}
	root, err := userdirs.DeepSeekProfileDir(profile)
	if err != nil {
		return nil, err
	}
	return planDeepSeek(s, commands, root, uninstall)
}

func planDeepSeek(s Scope, commands Commands, root string, uninstall bool) ([]Change, error) {
	bounded := nativeSpecialScope(s, root)
	dir := filepath.Join(root, "obituary-hooks")
	assets, err := planDeepSeekAssets(bounded, commands, dir, s.GuardRM, uninstall)
	if err != nil {
		return nil, err
	}
	rulesPath := filepath.Join(s.Project, "AGENTS.md")
	if s.Global {
		home, err := userdirs.DeepSeekDir()
		if err != nil {
			return nil, err
		}
		rulesPath = filepath.Join(home, "AGENTS.md")
	}
	// Profile exports share the home instruction file. Keep profile bundle
	// uninstall from removing instructions belonging to another profile.
	if s.Global {
		home, _ := userdirs.DeepSeekDir()
		if root != home {
			deepseekActivation(assets, dir, "obituary-deepseek-hooks", uninstall)
			return assets, nil
		}
	}
	rules, err := planPluginRules(bounded, rulesPath, "deepseek-harness", uninstall)
	if err != nil {
		return nil, err
	}
	deepseekActivation(assets, dir, "obituary-deepseek-hooks", uninstall)
	return OrderManagedInstallation(assets, []Change{rules}, uninstall)
}

func planDeepSeekAssets(bounded Scope, commands Commands, dir string, guardRM, uninstall bool) ([]Change, error) {
	hooksPath := filepath.Join(dir, "hooks.json")
	// The shipped Windows composition uses pwsh-sandbox for ctx.shell.
	// Pin the same executable used by the observer; PATH cannot select another.
	command := "& '" + strings.ReplaceAll(commands.Executable, "'", "''") + "' hook deepseek-harness"
	if !uninstall {
		if !filepath.IsAbs(commands.Executable) || !isObitMCP(commands.Executable, []string{"mcp"}) {
			return nil, fmt.Errorf("DeepSeek requires an absolute obit.exe path")
		}
		if !OwnedHookCommand(command, "deepseek-harness", nil) {
			return nil, fmt.Errorf("DeepSeek requires a literal PowerShell obit hook deepseek-harness command")
		}
	}
	spec, err := deepseekHookConfig(command, guardRM)
	if err != nil {
		return nil, err
	}
	var hooks []byte
	var hooksBefore *Change
	if !uninstall {
		ch, err := PlanHookConfig(bounded, hooksPath, spec, false)
		if err != nil {
			return nil, err
		}
		hooksBefore = &ch
		hooks = ch.data
		if ch.Action == "unchanged" {
			hooks = ch.orig
		}
	}
	patch, _ := deepseekFiles.ReadFile("plugins/deepseek/cordis.patch.yml")
	for token, value := range map[string]string{"__OBIT_HOOKS__": hooksPath, "__OBIT_OBSERVER__": "./index.ts", "__OBIT_EXECUTABLE__": commands.Executable} {
		raw, _ := marshal(value) // JSON quoted strings are valid YAML scalar values.
		patch = bytes.ReplaceAll(patch, []byte(token), raw)
	}
	observer, _ := deepseekFiles.ReadFile("plugins/deepseek/index.ts")
	transport, _ := pluginFiles.ReadFile("plugins/bridge/transport.ts")
	observer = bytes.ReplaceAll(observer, []byte("/* OBIT_BRIDGE */"), transport)
	manifest, _ := deepseekFiles.ReadFile("plugins/deepseek/package.json")
	assets, err := PlanManagedAssets(bounded, []ManagedAsset{
		{Path: hooksPath, ID: "deepseek-hooks", Owner: "deepseek-harness", Version: "1", Content: hooks},
		{Path: filepath.Join(dir, "index.ts"), ID: "deepseek-observer", Owner: "deepseek-harness", Version: "1", Content: observer},
		{Path: filepath.Join(dir, "cordis.patch.yml"), ID: "deepseek-hook-overlay", Owner: "deepseek-harness", Version: "1", Content: patch, Activation: true},
		{Path: filepath.Join(dir, "package.json"), ID: "deepseek-hook-bundle", Owner: "deepseek-harness", Version: "1", Content: manifest, Activation: true},
	}, uninstall)
	if err != nil {
		return nil, err
	}
	if hooksBefore != nil {
		if err := verifyDeepSeekHooks(assets, hooksPath, *hooksBefore); err != nil {
			return nil, err
		}
	}
	return assets, nil
}

func verifyDeepSeekHooks(assets []Change, hooksPath string, before Change) error {
	for _, ch := range assets {
		if ch.Path == hooksPath {
			if err := nativeSpecialOriginal(ch, before); err != nil {
				return err
			}
		}
	}
	return nil
}

func deepseekHookConfig(command string, guardRM bool) (HookConfig, error) {
	template, _ := deepseekFiles.ReadFile("plugins/deepseek/hooks.json")
	encoded, _ := marshal(command)
	template = bytes.ReplaceAll(template, []byte(`"__OBIT_COMMAND__"`), encoded)
	var config struct {
		Hooks map[string][]struct{ Hooks []json.RawMessage } `json:"hooks"`
	}
	if err := json.Unmarshal(template, &config); err != nil {
		return HookConfig{}, err
	}
	spec := HookConfig{Agent: "deepseek-harness", Shape: GroupedHooks}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "Stop"} {
		spec.Events = append(spec.Events, HookRegistration{Event: event, Handler: config.Hooks[event][0].Hooks[0]})
	}
	spec.Events = append(spec.Events, HookRegistration{Event: "PreToolUse", Matcher: "^(bash|pwsh)$",
		Handler: config.Hooks["Stop"][0].Hooks[0], Optional: !guardRM})
	return spec, nil
}

func deepseekActivation(changes []Change, dir, name string, uninstall bool) {
	if uninstall && PreservedError(changes) != nil {
		// An edited activation file may still reference every payload. Keep
		// the whole bundle when its complete removal cannot be established.
		for i := range changes {
			changes[i].Action, changes[i].Detail = "preserve", "Bundle contains an unowned or edited file; all bundle assets retained for manual removal"
		}
		return
	}
	quote := func(path string) string { return "'" + strings.ReplaceAll(path, "'", "''") + "'" }
	detail := fmt.Sprintf("Export only; activation unverified. For DSH 0.2.2-rc.1 install the matching official bridge with dsh plugin --profile NAME add @deepseek-ai/dsh-hooks-claude-code@0.2.2-rc.1, then dsh --profile NAME --patch %s (before app arguments), or dsh plugin --profile NAME add %s and restart. Select one bundle per process. Bundle < profile < home < CLI patch precedence; later rows replace the entire config. Keep obituary row IDs unique. Verify all observed tools use the local Windows filesystem and ctx.shell uses PowerShell, then set $env:OBIT_DEEPSEEK_LOCAL='1' and $env:OBIT_DEEPSEEK_SHELL='powershell'. Exported hook commands require PowerShell; custom bash compositions need manually reviewed hook launcher commands and OBIT_DEEPSEEK_SHELL=bash.", quote(filepath.Join(dir, "cordis.patch.yml")), quote(dir))
	if name == "obituary-deepseek-mcp" {
		detail = fmt.Sprintf("Export only; activation unverified. dsh --profile NAME --patch %s (before app arguments), or dsh plugin --profile NAME add %s and restart; select one obituary MCP bundle per process. dsh-mcp-client uses native stdio. Profile/home patches replace entire row configs; keep the obituary server name unique.", quote(filepath.Join(dir, "cordis.patch.yml")), quote(dir))
	}
	if uninstall {
		detail = fmt.Sprintf("Managed files only; external activation remains manual. Before removal stop the affected host, remove this --patch argument, or run dsh plugin --profile NAME remove %s for every profile that registered it. Keep provider rows and other plugins unchanged.", name)
	}
	for i := range changes {
		if changes[i].Action != "preserve" {
			changes[i].Detail = detail
		}
	}
}

func PlanDeepSeekMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	root, err := deepseekRoot(s)
	if err != nil {
		return nil, err
	}
	return planDeepSeekMCP(s, root, executable, uninstall)
}

func PlanDeepSeekProfileMCP(s Scope, profile, executable string, uninstall bool) ([]Change, error) {
	if !s.Global {
		return nil, fmt.Errorf("DeepSeek profile MCP requires global scope")
	}
	root, err := userdirs.DeepSeekProfileDir(profile)
	if err != nil {
		return nil, err
	}
	return planDeepSeekMCP(s, root, executable, uninstall)
}

func planDeepSeekMCP(s Scope, root, executable string, uninstall bool) ([]Change, error) {
	if !uninstall && (!filepath.IsAbs(executable) || !isObitMCP(executable, []string{"mcp"})) {
		return nil, fmt.Errorf("DeepSeek MCP requires an absolute obit.exe path")
	}
	entry, err := (CommandArgsMCP{}).Encode(executable, nil)
	if err != nil && !uninstall {
		return nil, err
	}
	var command struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	_ = json.Unmarshal(entry, &command)
	// This is a complete new, owned list, never a merge into the host YAML.
	patch, err := marshal([]any{map[string]any{"insert": []any{map[string]any{
		"id": "obituary-deepseek-mcp", "name": "@deepseek-ai/dsh-mcp-client",
		"config": map[string]any{"serverName": "obituary", "transport": "stdio", "command": command.Command, "args": command.Args},
	}}}})
	if err != nil {
		return nil, err
	}
	manifest, _ := marshal(map[string]any{"name": "obituary-deepseek-mcp", "version": "1.0.0", "private": true,
		"dsh":     map[string]any{"bundle": map[string]any{"patch": "./cordis.patch.yml"}},
		"engines": map[string]string{"dsh": "0.2.2-rc.1"}})
	dir := filepath.Join(root, "obituary-mcp")
	assets, err := PlanManagedAssets(nativeSpecialScope(s, root), []ManagedAsset{
		{Path: filepath.Join(dir, "cordis.patch.yml"), ID: "deepseek-mcp-overlay", Owner: "deepseek-harness", Version: "1", Content: append(patch, '\n'), Activation: true},
		{Path: filepath.Join(dir, "package.json"), ID: "deepseek-mcp-bundle", Owner: "deepseek-harness", Version: "1", Content: append(manifest, '\n'), Activation: true},
	}, uninstall)
	deepseekActivation(assets, dir, "obituary-deepseek-mcp", uninstall)
	return assets, err
}

func PlanDeepSeekSkill(s Scope, uninstall bool) ([]Change, error) {
	root, err := deepseekRoot(s)
	if err != nil {
		return nil, err
	}
	return PlanManagedAssets(nativeSpecialScope(s, root), []ManagedAsset{{Path: filepath.Join(root, "skills", "obituary", "SKILL.md"),
		ID: "deepseek-skill", Owner: "deepseek-harness", Version: "1", Content: skillContent}}, uninstall)
}

func init() {
	host, _ := hook.Lookup("deepseek-harness")
	RegisterAgent(Agent{Name: "deepseek-harness", Method: Plugin, Capabilities: host.Capabilities,
		Plan: PlanDeepSeek, PlanMCP: PlanDeepSeekMCP, PlanSkill: PlanDeepSeekSkill})
}
