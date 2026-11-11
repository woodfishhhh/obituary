package integrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// PlanMCP registers a native executable directly, without shell quoting or a
// wrapper. Only the obituary entry with exactly the mcp argument is ours.
func PlanMCP(s Scope, agent, executable string, uninstall bool) ([]Change, error) {
	if !uninstall && !isObitMCP(executable, []string{"mcp"}) {
		return nil, fmt.Errorf("MCP executable must be obit or an absolute path ending in obit.exe: %q", executable)
	}
	path, err := mcpPath(s, agent)
	if err != nil {
		return nil, err
	}
	if agent == "claude" && !s.Global {
		return planSharedMCP(s, path, agent, MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{Type: "stdio"}}, executable, uninstall)
	}
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return nil, err
	}
	if agent == "claude" {
		if s.Global {
			ch, err = planClaudeMCP(ch, executable, uninstall)
		} else {
			ch, err = planMCPJSON(ch, executable, uninstall)
		}
	} else {
		ch, err = planMCPTOML(ch, executable, uninstall)
	}
	return []Change{ch}, err
}

func mcpPath(s Scope, agent string) (string, error) {
	switch agent {
	case "claude":
		if s.Global {
			return userdirs.ClaudeMCPFile()
		}
		return filepath.Join(s.Project, ".mcp.json"), nil
	case "codex":
		dir := filepath.Join(s.Project, ".codex")
		if s.Global {
			var err error
			dir, err = userdirs.CodexDir()
			if err != nil {
				return "", err
			}
		}
		return filepath.Join(dir, "config.toml"), nil
	default:
		return "", fmt.Errorf("unsupported agent %q", agent)
	}
}

func readIntegrationChange(s Scope, path string) (Change, error) {
	ch := Change{Path: path, Action: "unchanged"}
	if !s.Global {
		ch.project = s.Project
		if err := s.checkInside(s.Project, path); err != nil {
			return ch, err
		}
		backup, err := projectBackup(s, path)
		if err != nil {
			return ch, err
		}
		ch.backup = &backup
	}
	if s.inspection != nil {
		return s.readIntegrationPreview(ch)
	}
	p, err := openIntegrationTarget(ch, false)
	if err != nil {
		return ch, err
	}
	defer p.root.Close()
	ch.location = &p.location
	data, info, err := p.read()
	ch.identity = info
	ch.orig, ch.existed = data, err == nil
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return ch, err
}

func setIntegrationData(ch Change, data []byte) Change {
	if bytes.Equal(ch.orig, data) {
		return ch
	}
	ch.Action, ch.data = "update", data
	if !ch.existed {
		ch.Action = "create"
	}
	return ch
}

func isObitMCP(command string, args []string) bool {
	if len(args) != 1 || args[0] != "mcp" {
		return false
	}
	base := strings.ToLower(filepath.Base(command))
	return (base == "obit" || base == "obit.exe") && (filepath.IsAbs(command) || strings.EqualFold(command, base))
}

func planMCPJSON(ch Change, executable string, uninstall bool) (Change, error) {
	root, err := parseObject(ch.orig)
	if err != nil {
		return ch, fmt.Errorf("%s: %w", ch.Path, err)
	}
	servers := newObject()
	if raw, ok := root.get("mcpServers"); ok {
		servers, err = parseObject(raw)
		if err != nil {
			return ch, fmt.Errorf("%s: mcpServers: %w", ch.Path, err)
		}
	}
	entry := newObject()
	if raw, exists := servers.get("obituary"); exists {
		var registered struct {
			Command string
			Args    []string
			Type    string
		}
		ours := json.Unmarshal(raw, &registered) == nil && isObitMCP(registered.Command, registered.Args) && (registered.Type == "" || registered.Type == "stdio")
		if !ours {
			if uninstall {
				ch.Detail = "unowned MCP entry preserved"
				return ch, nil
			}
			return ch, fmt.Errorf("%s: mcpServers.obituary is not an Obituary stdio registration; left untouched", ch.Path)
		}
		entry, err = parseObject(raw)
		if err != nil {
			return ch, err
		}
	} else if uninstall {
		return ch, nil
	}
	if uninstall {
		servers.remove("obituary")
	} else {
		_ = entry.set("type", "stdio")
		_ = entry.set("command", executable)
		_ = entry.set("args", []string{"mcp"})
		_ = servers.set("obituary", entry)
	}
	if servers.empty() {
		root.remove("mcpServers")
	} else {
		_ = root.set("mcpServers", servers)
	}
	data, err := root.pretty()
	if err != nil {
		return ch, err
	}
	if sameJSON(bytes.TrimPrefix(ch.orig, []byte("\xef\xbb\xbf")), data) {
		return ch, nil
	}
	if bytes.HasPrefix(ch.orig, []byte("\xef\xbb\xbf")) {
		data = append([]byte("\xef\xbb\xbf"), data...)
	}
	ch.Detail = "MCP stdio registration"
	return setIntegrationData(ch, data), nil
}
