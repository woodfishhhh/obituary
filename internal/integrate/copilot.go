package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// GitHub's hooks/MCP/agent-skills references define these paths. The current
// VS Code Copilot harness shares these portable locations; legacy Local harness
// profiles are separate.
func copilotDirectory(s Scope) (string, error) {
	if s.Global {
		return userdirs.CopilotDir()
	}
	if !filepath.IsAbs(s.Project) {
		return "", fmt.Errorf("copilot requires an absolute project directory")
	}
	return filepath.Join(s.Project, ".github"), nil
}

// PlanCopilot installs only one spelling per event. Registering both native and
// compatible spellings makes the host execute a hook twice.
var copilotAdapter = hookAdapter{
	directory: copilotDirectory, file: "hooks/obituary.json",
	config: HookConfig{Agent: "copilot", Shape: FlatHooks, CommandKeys: []string{"powershell", "bash", "command"}, EventArgs: []string{},
		Defaults: json.RawMessage(`{"version":1}`),
		Events:   []HookRegistration{{Event: "sessionStart"}, {Event: "postToolUse"}, {Event: "agentStop"}, {Event: "sessionEnd"}, {Event: "preToolUse"}}},
	removeEvents: []HookRegistration{{Event: "SessionStart"}, {Event: "PostToolUse"}, {Event: "Stop"}, {Event: "SessionEnd"}, {Event: "PreToolUse"}},
	guardEvent:   "preToolUse", handler: copilotHookHandler,
	check:     func(ch Change) error { return hookVersion(ch, "Copilot") },
	rulesFile: "copilot-instructions.md",
}

func PlanCopilot(s Scope, commands Commands, uninstall bool) ([]Change, error) {
	return copilotAdapter.plan(s, commands, uninstall)
}

func copilotHookHandler(commands Commands, event string) (json.RawMessage, error) {
	handler := map[string]any{"type": "command", "powershell": powershellHookCommand(commands.Native) + " --event " + event, "timeoutSec": 5}
	if commands.POSIX != "" {
		handler["bash"] = commands.POSIX + " --event " + event
	}
	if event == "preToolUse" {
		handler["matcher"] = "bash|powershell"
	}
	if event == "postToolUse" {
		handler["matcher"] = "create|edit|Write|Edit"
	}
	return marshal(handler)
}

// copilotMCP retains an existing local/stdio spelling and user tool filters.
// GitHub's schema also uses tools:["*"] to expose all server tools on creation.
type copilotMCP struct{}

func (copilotMCP) normalized(raw json.RawMessage) (json.RawMessage, bool) {
	obj, err := parseObject(raw)
	if err != nil {
		return nil, false
	}
	local := sameConfigJSON(obj.values["type"], []byte(`"local"`))
	if local {
		_ = obj.set("type", "stdio")
	}
	data, _ := obj.MarshalJSON()
	return data, local
}

func (c copilotMCP) Owned(raw json.RawMessage) bool {
	normalized, _ := c.normalized(raw)
	return normalized != nil && (CommandArgsMCP{}).Owned(normalized)
}

func (c copilotMCP) Encode(executable string, existing json.RawMessage) (json.RawMessage, error) {
	var local bool
	if existing != nil {
		if !c.Owned(existing) {
			return nil, fmt.Errorf("unowned Copilot MCP entry")
		}
		existing, local = c.normalized(existing)
	}
	raw, err := (CommandArgsMCP{Type: "stdio"}).Encode(executable, existing)
	if err != nil {
		return nil, err
	}
	obj, err := parseObject(raw)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		_ = obj.set("tools", []string{"*"})
	}
	if local {
		_ = obj.set("type", "local")
	}
	return obj.MarshalJSON()
}

func PlanCopilotMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	dir, err := copilotDirectory(s)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "mcp-config.json")
	if !s.Global {
		path = filepath.Join(s.Project, ".mcp.json")
	}
	paths := []string{path}
	if !s.Global {
		paths = append(paths, filepath.Join(dir, "mcp.json"))
	}
	var changes []Change
	for index, path := range paths {
		plan, err := planCopilotMCPFile(s, path, executable, index > 0, uninstall)
		if err != nil {
			return nil, err
		}
		changes = append(changes, plan...)
	}
	return changes, nil
}

func planCopilotMCPFile(s Scope, path, executable string, secondary, uninstall bool) ([]Change, error) {
	before, err := readIntegrationChange(s, path)
	if err != nil {
		return nil, err
	}
	if err := nonemptyConfig(before); err != nil {
		return nil, err
	}
	entry, bare, err := copilotMCPEntry(s, before)
	if err != nil {
		return nil, err
	}
	// A secondary definition would become active again after uninstall.
	// Refresh/remove an existing one, but never create a duplicate there.
	if secondary && entry == nil {
		return []Change{before}, nil
	}
	if bare {
		return planCopilotBareMCP(s, before, executable, entry, uninstall)
	}
	plan, err := planSharedMCP(s, path, "copilot", MCPConfig{ServersPath: []string{"mcpServers"}, Codec: copilotMCP{}}, executable, uninstall)
	if err != nil {
		return nil, err
	}
	if err := checkPlannedOriginal(plan, before); err != nil {
		return nil, err
	}
	return plan, nil
}

func copilotMCPEntry(s Scope, before Change) (json.RawMessage, bool, error) {
	data, err := configJSON(before.orig, ConfigJSON)
	if err != nil {
		return nil, false, err
	}
	root, _ := parseObject(data)
	_, wrapped := root.get("mcpServers")
	if s.Global && !wrapped && !root.empty() {
		return nil, false, fmt.Errorf("%s requires a mcpServers container; manually wrap the existing server entries under mcpServers without dropping entries, then retry", before.Path)
	}
	bare := !s.Global && !wrapped && !root.empty()
	entryPath := []string{"mcpServers", "obituary"}
	if bare {
		entryPath = []string{"obituary"}
	}
	entry, err := configMember(data, entryPath)
	return entry, bare, err
}

func planCopilotBareMCP(s Scope, before Change, executable string, entry json.RawMessage, uninstall bool) ([]Change, error) {
	// GitHub's project configuration also accepts a root server map. Reuse
	// the shared codec and checked member editor without a parent container.
	codec := copilotMCP{}
	if entry != nil && !codec.Owned(entry) {
		if !uninstall {
			return nil, fmt.Errorf("%s: obituary is not an owned stdio registration; left untouched", before.Path)
		}
		before.Action, before.Detail = "preserve", "unowned MCP entry preserved"
		return []Change{before}, nil
	}
	edit := ConfigEdit{Path: []string{"obituary"}, Remove: uninstall, CheckExpected: true, Expected: entry}
	if !uninstall {
		var err error
		edit.Value, err = codec.Encode(executable, entry)
		if err != nil {
			return nil, err
		}
	}
	change, err := PlanConfigEdits(s, before.Path, ConfigJSON, []ConfigEdit{edit})
	change.Detail = "Copilot project MCP root server map"
	if err != nil {
		return nil, err
	}
	if change.existed != before.existed || !bytes.Equal(change.orig, before.orig) {
		return nil, fmt.Errorf("%s changed while planning Copilot MCP; retry", before.Path)
	}
	return []Change{change}, nil
}

func PlanCopilotSkill(s Scope, uninstall bool) ([]Change, error) {
	dir, err := copilotDirectory(s)
	if err != nil {
		return nil, err
	}
	return PlanManagedAssets(s, []ManagedAsset{{Path: filepath.Join(dir, "skills", "obituary", "SKILL.md"),
		ID: "copilot-obituary-skill", Owner: "copilot", Version: "1", Content: skillContent}}, uninstall)
}

func init() {
	host, _ := hook.Lookup("copilot")
	capabilities := host.Capabilities
	capabilities.MCP = hook.Support{Level: hook.Limited, Detail: "Portable mcpServers for CLI/current VS Code Copilot harness; legacy Local harness profile configuration remains manual"}
	capabilities.Skill = hook.Support{Level: hook.Supported, Detail: ".github/skills or COPILOT_HOME/skills"}
	RegisterAgent(Agent{Name: "copilot", Method: NativeHooks, Capabilities: capabilities, Plan: PlanCopilot, PlanMCP: PlanCopilotMCP, PlanSkill: PlanCopilotSkill})
}
