package integrate

import (
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func PlanKimiMCP(s Scope, executable string, uninstall bool) ([]Change, error) {
	if !s.Global {
		path := filepath.Join(s.Project, ".kimi", "mcp.json")
		if uninstall {
			return nil, manualf("Kimi project MCP is manually loaded: remove mcpServers.obituary from the file passed to --mcp-config-file after verifying command is obit.exe and args is [\"mcp\"]; review the launch flag separately")
		}
		entry, _ := marshal(map[string]any{"mcpServers": map[string]any{"obituary": map[string]any{"command": executable, "args": []string{"mcp"}}}})
		return nil, manualf("Kimi project MCP needs an explicit launch flag: merge %s into %s after checking ownership, then start kimi --mcp-config-file %q; ordinary project MCP files are not auto-loaded", entry, path, path)
	}
	root, err := userdirs.KimiDir()
	if err != nil {
		return nil, err
	}
	ch, err := PlanMCPConfig(nativeSpecialScope(s, root), filepath.Join(root, "mcp.json"), MCPConfig{ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
	return []Change{ch}, err
}
