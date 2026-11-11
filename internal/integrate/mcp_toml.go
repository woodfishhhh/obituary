package integrate

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

const (
	mcpBlockBegin = "# obit:mcp:begin (managed by obit init)"
	mcpBlockEnd   = "# obit:mcp:end"
)

func planMCPTOML(ch Change, executable string, uninstall bool) (Change, error) {
	bom := []byte("\xef\xbb\xbf")
	data := bytes.TrimPrefix(ch.orig, bom)
	var full map[string]any
	if err := toml.Unmarshal(data, &full); err != nil {
		return ch, fmt.Errorf("%s: invalid TOML: %w", ch.Path, err)
	}
	start, end, err := mcpBlockRange(data)
	if err != nil {
		return ch, fmt.Errorf("%s: %w", ch.Path, err)
	}
	servers, _ := full["mcp_servers"].(map[string]any)
	if start < 0 {
		return planUnmarkedMCPTOML(ch, data, servers, executable, uninstall)
	}
	block := data[start:end]
	if err := checkMCPTOMLOwnership(ch.Path, block, servers); err != nil {
		return ch, err
	}
	if uninstall {
		data, err = removeMCPTOMLBlock(ch.Path, data, full, servers, start, end)
		if err != nil {
			return ch, err
		}
	} else {
		updated, err := refreshMCPCommand(block, executable)
		if err != nil {
			return ch, err
		}
		data = append(append(bytes.Clone(data[:start]), updated...), data[end:]...)
	}
	return renderMCPTOML(ch, data)
}

func removeMCPTOMLBlock(path string, data []byte, full, servers map[string]any, start, end int) ([]byte, error) {
	data = append(bytes.Clone(data[:start]), data[end:]...)
	delete(servers, "obituary")
	var remaining map[string]any
	if err := toml.Unmarshal(data, &remaining); err != nil {
		return nil, err
	}
	// Detect keys/tables outside the markers whose meaning would change
	// after removal (TOML table scope extends beyond comment markers).
	if len(servers) == 0 {
		delete(full, "mcp_servers")
	}
	if rest, ok := remaining["mcp_servers"].(map[string]any); ok && len(rest) == 0 {
		delete(remaining, "mcp_servers")
	}
	if !reflect.DeepEqual(full, remaining) {
		return nil, fmt.Errorf("%s: removing the MCP block would change user TOML", path)
	}
	return data, nil
}

func planUnmarkedMCPTOML(ch Change, data []byte, servers map[string]any, executable string, uninstall bool) (Change, error) {
	if uninstall {
		if _, exists := servers["obituary"]; exists {
			ch.Detail = "unowned MCP entry preserved"
		}
		return ch, nil // never claim a user's unmarked table
	}
	if _, exists := servers["obituary"]; exists {
		return ch, fmt.Errorf("%s: unmarked mcp_servers.obituary belongs to the user", ch.Path)
	}
	if inlineMCPParent(data) {
		return ch, fmt.Errorf("%s: mcp_servers is an inline table; it cannot be extended with an obituary subtable", ch.Path)
	}
	body, err := toml.Marshal(map[string]any{"command": executable, "args": []string{"mcp"}})
	if err != nil {
		return ch, err
	}
	block := []byte(mcpBlockBegin + "\n[mcp_servers.obituary]\n" + string(body) + mcpBlockEnd + "\n")
	// Append to preserve root assignments and EOF without a newline.
	// The begin marker's range owns the one additional LF separator.
	block = append([]byte("\n"), block...)
	data = append(bytes.Clone(data), block...)
	return renderMCPTOML(ch, data)
}

func checkMCPTOMLOwnership(path string, block []byte, servers map[string]any) error {
	var owned map[string]any
	if err := toml.Unmarshal(block, &owned); err != nil {
		return err
	}
	ownedServers, _ := owned["mcp_servers"].(map[string]any)
	entry, _ := ownedServers["obituary"].(map[string]any)
	command, _ := entry["command"].(string)
	var args []string
	if values, ok := entry["args"].([]any); ok {
		for _, value := range values {
			arg, ok := value.(string)
			if !ok {
				return fmt.Errorf("%s: non-string MCP argument", path)
			}
			args = append(args, arg)
		}
	}
	if len(owned) != 1 || len(ownedServers) != 1 || !isObitMCP(command, args) || !reflect.DeepEqual(entry, servers["obituary"]) {
		return fmt.Errorf("%s: edited MCP ownership block; left untouched", path)
	}
	return nil
}

func renderMCPTOML(ch Change, data []byte) (Change, error) {
	var check map[string]any
	if err := toml.Unmarshal(data, &check); err != nil {
		return ch, fmt.Errorf("%s: proposed MCP configuration is invalid: %w", ch.Path, err)
	}
	if bytes.HasPrefix(ch.orig, []byte("\xef\xbb\xbf")) {
		data = append(bytes.Clone([]byte("\xef\xbb\xbf")), data...)
	}
	ch.Detail = "owned MCP TOML block"
	return setIntegrationData(ch, data), nil
}

// Only actual standalone TOML comments are markers; strings containing marker
// text do not grant ownership. The installer owns one leading LF separator.
func mcpBlockRange(data []byte) (start, end int, err error) {
	start, end = -1, -1
	p := unstable.Parser{KeepComments: true}
	p.Reset(data)
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind != unstable.Comment {
			continue
		}
		raw := string(p.Raw(n.Raw))
		offset := int(n.Raw.Offset)
		if offset > 0 && data[offset-1] != '\n' {
			continue
		}
		switch strings.TrimSuffix(raw, "\r") {
		case mcpBlockBegin:
			if start >= 0 || end >= 0 {
				return -1, -1, fmt.Errorf("duplicate MCP ownership markers")
			}
			start = offset
			if start > 0 && data[start-1] == '\n' {
				start--
				if start > 0 && data[start-1] == '\r' {
					start--
				}
			}
		case mcpBlockEnd:
			if start < 0 || end >= 0 {
				return -1, -1, fmt.Errorf("unpaired MCP ownership markers")
			}
			end = offset + int(n.Raw.Length)
			if end < len(data) && data[end] == '\r' {
				end++
			}
			if end < len(data) && data[end] == '\n' {
				end++
			}
		}
	}
	if err := p.Error(); err != nil {
		return -1, -1, err
	}
	if start >= 0 && end < 0 {
		return -1, -1, fmt.Errorf("unclosed MCP ownership block")
	}
	return start, end, nil
}

func inlineMCPParent(data []byte) bool {
	p := unstable.Parser{}
	p.Reset(data)
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind == unstable.Table || n.Kind == unstable.ArrayTable {
			break
		}
		if n.Kind != unstable.KeyValue || n.Value().Kind != unstable.InlineTable {
			continue
		}
		keys := n.Key()
		if keys.Next() && string(keys.Node().Data) == "mcp_servers" && !keys.Next() {
			return true
		}
	}
	return false
}

func refreshMCPCommand(block []byte, executable string) ([]byte, error) {
	p := unstable.Parser{}
	p.Reset(block)
	var table []string
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind == unstable.Table {
			table = nil
			it := n.Key()
			for it.Next() {
				table = append(table, string(it.Node().Data))
			}
		}
		if n.Kind != unstable.KeyValue || !reflect.DeepEqual(table, []string{"mcp_servers", "obituary"}) {
			continue
		}
		key := n.Key()
		if !key.Next() || string(key.Node().Data) != "command" || key.Next() {
			continue
		}
		// Marshal just a string value using TOML's own escaping rules.
		encoded, err := toml.Marshal(map[string]string{"command": executable})
		if err != nil {
			return nil, err
		}
		_, value, _ := strings.Cut(string(encoded), "=")
		r := n.Value().Raw
		out := append(bytes.Clone(block[:r.Offset]), strings.TrimSpace(value)...)
		return append(out, block[r.Offset+r.Length:]...), nil
	}
	return nil, fmt.Errorf("owned MCP block has no directly editable command")
}
