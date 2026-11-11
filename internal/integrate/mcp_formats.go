package integrate

import (
	"encoding/json"
	"fmt"
	"slices"
)

// MCPEntryCodec describes a host's stdio entry. Encode refreshes an owned entry
// while retaining fields such as env. A nil existing entry requests creation.
type MCPEntryCodec interface {
	Owned(json.RawMessage) bool
	Encode(executable string, existing json.RawMessage) (json.RawMessage, error)
}

// CommandArgsMCP is the command:string / args:array schema. Type adds the host's
// optional type:"stdio" field on creation; the empty value omits that field.
type CommandArgsMCP struct{ Type string }

func (codec CommandArgsMCP) Owned(raw json.RawMessage) bool {
	var command string
	var args []string
	obj, err := parseObject(raw)
	if err != nil || json.Unmarshal(obj.values["command"], &command) != nil || json.Unmarshal(obj.values["args"], &args) != nil || !isObitMCP(command, args) {
		return false
	}
	for _, key := range []string{"type", "transport"} {
		if value, exists := obj.get(key); exists && !sameConfigJSON(value, []byte(`"stdio"`)) {
			return false
		}
	}
	return true
}

func (codec CommandArgsMCP) Encode(executable string, existing json.RawMessage) (json.RawMessage, error) {
	if codec.Type != "" && codec.Type != "stdio" {
		return nil, fmt.Errorf("command/args MCP type must be stdio")
	}
	if !isObitMCP(executable, []string{"mcp"}) || existing != nil && !codec.Owned(existing) {
		return nil, fmt.Errorf("unowned MCP command/args entry")
	}
	entry, err := parseObject(existing)
	if err != nil {
		return nil, err
	}
	if existing == nil && codec.Type != "" {
		_ = entry.set("type", codec.Type)
	}
	_ = entry.set("command", executable)
	_ = entry.set("args", []string{"mcp"})
	return entry.MarshalJSON()
}

// CommandArrayMCP is OpenCode/Kilo's type:"local", command:[exe,"mcp"] schema.
type CommandArrayMCP struct{}

func (CommandArrayMCP) Owned(raw json.RawMessage) bool {
	var kind string
	var command []string
	obj, err := parseObject(raw)
	return err == nil && json.Unmarshal(obj.values["type"], &kind) == nil && kind == "local" && json.Unmarshal(obj.values["command"], &command) == nil && len(command) == 2 && isObitMCP(command[0], command[1:])
}

func (codec CommandArrayMCP) Encode(executable string, existing json.RawMessage) (json.RawMessage, error) {
	if !isObitMCP(executable, []string{"mcp"}) || existing != nil && !codec.Owned(existing) {
		return nil, fmt.Errorf("unowned MCP command-array entry")
	}
	entry, err := parseObject(existing)
	if err != nil {
		return nil, err
	}
	_ = entry.set("type", "local")
	_ = entry.set("command", []string{executable, "mcp"})
	return entry.MarshalJSON()
}

// MCPConfig separates the configuration file from its server container and
// entry codec. Examples: ["mcpServers"], ["servers"], ["mcp"],
// ["mcp","servers"], ["mcp_servers"].
type MCPConfig struct {
	Format      ConfigFormat
	ServersPath []string
	Codec       MCPEntryCodec
}

func PlanMCPConfig(s Scope, path string, spec MCPConfig, executable string, uninstall bool) (Change, error) {
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	if len(spec.ServersPath) == 0 || spec.Codec == nil {
		return ch, fmt.Errorf("MCP configuration needs a server path and entry codec")
	}
	if spec.Format == "" {
		spec.Format = ConfigJSON
	}
	entryPath := append(slices.Clone(spec.ServersPath), "obituary")
	ch.preservePath = entryPath
	if spec.Format == ConfigJSON5 {
		return planMCPJSON5(ch, spec, entryPath, executable, uninstall)
	}
	data, err := configJSON(ch.orig, spec.Format)
	if err != nil {
		return ch, fmt.Errorf("%s: %w", path, err)
	}
	entry, err := configMember(data, entryPath)
	if err != nil {
		return ch, err
	}
	if entry != nil && !spec.Codec.Owned(entry) {
		if uninstall {
			ch.Action, ch.Detail = "preserve", "unowned MCP entry preserved"
			return ch, nil
		}
		return ch, fmt.Errorf("%s: obituary is not an owned stdio registration; left untouched", path)
	}
	if entry == nil && uninstall {
		return ch, nil
	}
	edit := ConfigEdit{Path: entryPath, Remove: uninstall, Expected: entry, CheckExpected: true}
	if !uninstall {
		edit.Value, err = spec.Codec.Encode(executable, entry)
		if err != nil {
			return ch, err
		}
	}
	out, err := editConfig(ch.orig, spec.Format, []ConfigEdit{edit})
	if err != nil {
		return ch, err
	}
	ch.Detail = "MCP stdio registration"
	return setIntegrationData(ch, out), nil
}

func planMCPJSON5(ch Change, spec MCPConfig, entryPath []string, executable string, uninstall bool) (Change, error) {
	edit := ConfigEdit{Path: entryPath, Remove: uninstall}
	if !uninstall {
		var err error
		edit.Value, err = spec.Codec.Encode(executable, nil)
		if err != nil {
			return ch, err
		}
	}
	_, err := editConfig(ch.orig, spec.Format, []ConfigEdit{edit})
	return ch, fmt.Errorf("%s: %w", ch.Path, err)
}

func configMember(data []byte, path []string) (json.RawMessage, error) {
	for _, key := range path {
		obj, err := parseObject(data)
		if err != nil {
			return nil, fmt.Errorf("%s: parent is not an object: %w", key, err)
		}
		raw, exists := obj.get(key)
		if !exists {
			return nil, nil
		}
		data = raw
	}
	return json.RawMessage(data), nil
}
