package integrate

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// Each capability has its own bundle, so independent Plan/PlanMCP/PlanSkill
// calls never race to replace a common manifest. Shared ownership preserves
// user-edited assets; checked installation plans own all file writes.
//
//go:embed plugins/minimax/.minimax-plugin/plugin.json plugins/minimax/hooks/hooks.json plugins/minimax/icon.png plugins/minimax/mcp.mjs
var minimaxFiles embed.FS

const minimaxAgent = "minimax-code"

func minimaxGlobalRoot(s Scope) (Scope, string, error) {
	root, err := userdirs.MiniMaxDir()
	return nativeSpecialScope(s, root), root, err
}

// PlanMiniMax targets the native local plugin reader at 3c21163c. Project
// plugins have no verified discovery location; project scope installs AGENTS.md.
func PlanMiniMax(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	if !s.Global {
		if !filepath.IsAbs(s.Project) {
			return nil, fmt.Errorf("MiniMax Code requires an absolute project directory")
		}
		ch, err := planPluginRules(s, filepath.Join(s.Project, "AGENTS.md"), minimaxAgent, uninstall)
		ch.Detail = "Project AGENTS.md only; native hooks load from MINIMAX_DATA_DIR/plugins; use a global installation for hooks"
		if s.GuardRM && !uninstall {
			ch.Detail += "; install PreToolUse with obit init -g --agent minimax-code --guard-rm"
		}
		return []Change{ch}, err
	}
	bounded, root, err := minimaxGlobalRoot(s)
	if err != nil {
		return nil, err
	}
	command := ""
	if !uninstall {
		if !filepath.IsAbs(commands.Executable) || !isObitMCP(commands.Executable, []string{"mcp"}) || strings.ContainsAny(commands.Executable, "%!\"&|^<>`$\x00\r\n") {
			return nil, fmt.Errorf("MiniMax commandWindows requires a literal absolute obit.exe path without cmd.exe expansion characters")
		}
		command = `"` + commands.Executable + `" hook ` + minimaxAgent
		if !OwnedHookCommand(command, minimaxAgent, nil) {
			return nil, fmt.Errorf("invalid MiniMax hook command")
		}
	}
	payload, err := minimaxFiles.ReadFile("plugins/minimax/hooks/hooks.json")
	if err != nil {
		return nil, err
	}
	encoded, err := marshal(command)
	if err != nil {
		return nil, err
	}
	payload = bytes.ReplaceAll(payload, []byte(`"__OBIT_COMMAND_WINDOWS__"`), encoded)
	if !s.GuardRM && !uninstall {
		payload, err = miniMaxOptionalGuard(bounded, root, payload)
		if err != nil {
			return nil, err
		}
	}
	return planMiniMaxBundle(bounded, root, "obituary", "hooks", map[string][]byte{"hooks/hooks.json": payload}, uninstall)
}

func miniMaxOptionalGuard(s Scope, root string, payload []byte) ([]byte, error) {
	old, err := readIntegrationChange(s, filepath.Join(root, "plugins", "obituary", "hooks", "hooks.json"))
	if err != nil {
		return nil, err
	}
	guard, err := configMember(old.orig, []string{"hooks", "PreToolUse"})
	if err != nil {
		return nil, err
	}
	if guard != nil {
		return payload, nil // A normal refresh retains an earlier opt-in.
	}
	return editConfig(payload, ConfigJSON, []ConfigEdit{{Path: []string{"hooks", "PreToolUse"}, Remove: true}})
}

func PlanMiniMaxMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	if !s.Global {
		return planMiniMaxProjectMCP(s, executable, uninstall)
	}
	bounded, root, err := minimaxGlobalRoot(s)
	if err != nil {
		return nil, err
	}
	var runtime []byte
	if !uninstall {
		if !filepath.IsAbs(executable) {
			return nil, fmt.Errorf("MiniMax MCP requires an absolute obit.exe path")
		}
		runtime, err = (CommandArgsMCP{}).Encode(executable, nil)
		if err != nil {
			return nil, err
		}
	}
	wrapper, err := minimaxFiles.ReadFile("plugins/minimax/mcp.mjs")
	if err != nil {
		return nil, err
	}
	// Native V1 forbids an absolute command or argument, including local
	// packages. A Node stdio relay pins the executable in its own owned file.
	mcp := []byte("{\"schemaVersion\":1,\"mcpServers\":{\"obituary\":{\"type\":\"stdio\",\"command\":\"node\",\"args\":[\"./mcp.mjs\"]}}}\n")
	return planMiniMaxBundle(bounded, root, "obituary-mcp", "mcpServers", map[string][]byte{"obituary.mcp.json": mcp, "mcp.mjs": wrapper, "runtime.json": runtime}, uninstall)
}

func PlanMiniMaxSkill(s Scope, uninstall bool) ([]Change, error) {
	if !s.Global {
		return nil, manualf("MiniMax project Skill discovery is unverified; use the global obituary-skill local plugin with the selected MINIMAX_DATA_DIR")
	}
	bounded, root, err := minimaxGlobalRoot(s)
	if err != nil {
		return nil, err
	}
	return planMiniMaxBundle(bounded, root, "obituary-skill", "skills", map[string][]byte{"skills/obituary/SKILL.md": skillContent}, uninstall)
}

func planMiniMaxBundle(s Scope, root, name, capability string, files map[string][]byte, uninstall bool) ([]Change, error) {
	dir := filepath.Join(root, "plugins", name)
	manifest, err := minimaxFiles.ReadFile("plugins/minimax/.minimax-plugin/plugin.json")
	if err != nil {
		return nil, err
	}
	refs := map[string]string{"hooks": "hooks/hooks.json", "mcpServers": "obituary.mcp.json", "skills": "skills/obituary/SKILL.md"}
	ref, ok := refs[capability]
	if !ok {
		return nil, fmt.Errorf("unsupported MiniMax capability %q", capability)
	}
	value, _ := marshal(name)
	reference, _ := marshal([]string{ref})
	edits := []ConfigEdit{{Path: []string{"name"}, Value: value}, {Path: []string{"hooks"}, Value: json.RawMessage(`[]`)}, {Path: []string{capability}, Value: reference}}
	manifest, err = editConfig(manifest, ConfigJSON, edits)
	if err != nil {
		return nil, err
	}
	icon, err := minimaxFiles.ReadFile("plugins/minimax/icon.png")
	if err != nil {
		return nil, err
	}
	files["icon.png"] = icon
	var assets []ManagedAsset
	for path, content := range files {
		assets = append(assets, ManagedAsset{Path: filepath.Join(dir, filepath.FromSlash(path)), ID: "minimax-" + name + ":" + path, Owner: minimaxAgent, Version: "1", Content: content})
	}
	slices.SortFunc(assets, func(a, b ManagedAsset) int { return strings.Compare(a.Path, b.Path) })
	assets = append(assets, ManagedAsset{Path: filepath.Join(dir, ".minimax-plugin", "plugin.json"), ID: "minimax-" + name + ":manifest", Owner: minimaxAgent, Version: "1", Content: manifest, Activation: true})
	changes, err := PlanManagedAssets(s, assets, uninstall)
	if err != nil {
		return nil, err
	}
	return miniMaxBundleDetails(changes, assets[len(assets)-1].Path, name, capability, uninstall), nil
}

func init() {
	host, _ := hook.Lookup(minimaxAgent)
	RegisterAgent(Agent{Name: minimaxAgent, Method: Plugin, Capabilities: host.Capabilities, Plan: PlanMiniMax, PlanMCP: PlanMiniMaxMCP, PlanSkill: PlanMiniMaxSkill})
}

func planMiniMaxProjectMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	if !filepath.IsAbs(s.Project) {
		return nil, fmt.Errorf("MiniMax Code requires an absolute project directory")
	}
	path := filepath.Join(s.Project, ".mcp.json")
	before, err := readIntegrationChange(s, path)
	if err != nil {
		return nil, err
	}
	// mcode uses JSON.parse without a BOM/comment preprocessor. Keep
	// unsupported user bytes intact instead of claiming an active install.
	if before.existed && !json.Valid(before.orig) {
		return nil, fmt.Errorf("%s: MiniMax requires nonempty strict UTF-8 JSON without BOM or comments; file retained", path)
	}
	if err := miniMaxProjectMCPOwnership(s, before, uninstall); err != nil {
		return nil, err
	}
	changes, err := planPluginMCP(s, path, minimaxAgent, executable, uninstall)
	if err != nil {
		return nil, err
	}
	for i := range changes {
		if changes[i].Path == path {
			if err := nativeSpecialOriginal(changes[i], before); err != nil {
				return nil, err
			}
			if changes[i].Action != "preserve" {
				changes[i].Detail = "Project MCP requires MiniMax's digest-bound project approval; Obituary does not grant trust"
			}
		}
	}
	return changes, nil

}

func miniMaxProjectMCPOwnership(s Scope, before Change, uninstall bool) error {
	if uninstall || !before.existed {
		return nil
	}
	entry, err := configMember(before.orig, []string{"mcpServers", "obituary"})
	if err != nil || entry == nil {
		return err
	}
	receipt, err := readSharedReceipt(s, before.Path+".obituary-owners")
	if err != nil {
		return err
	}
	if !receipt.existed {
		return fmt.Errorf("%s: existing obituary registration has no shared owner receipt; coordinate its current host before adopting it", before.Path)
	}
	return nil
}

func miniMaxBundleDetails(changes []Change, manifestPath, name, capability string, uninstall bool) []Change {
	if uninstall && slices.ContainsFunc(changes, func(ch Change) bool { return ch.Path == manifestPath && ch.Action == "preserve" }) {
		// An edited activation entry may still load any of its dependencies.
		for i := range changes {
			if changes[i].existed {
				changes[i].Action, changes[i].Detail = "preserve", "MiniMax activation manifest is unverified; bundle retained"
				changes[i].delete, changes[i].next = false, nil
			}
		}
		return changes
	}
	for i := range changes {
		if changes[i].Path == manifestPath && changes[i].Action != "preserve" {
			changes[i].Detail = "Local native bundle; mcode plugin marketplace upgrade, then mcode plugin enable " + name + "@local; verify mcode plugin list --json; stop active sessions before refresh/uninstall because hooks use immutable cached snapshots"
			if uninstall {
				changes[i].Detail = "Owned local bundle removed; refresh mcode plugin marketplace upgrade after active sessions stop; host runtime caches are retained"
			} else if capability == "mcpServers" {
				changes[i].Detail += "; node must be available in the host PATH; the relay pins obit.exe without a shell"
			}
		}
	}
	return changes
}
