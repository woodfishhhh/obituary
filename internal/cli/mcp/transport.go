package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/woodfishhhh/obituary/internal/core"
)

func addMCPTool[I any](server *mcp.Server, svc *core.Service, name, description string, readOnly bool, run func(*core.Service, I) mcpReply) {
	// SDK v1.8.0 tool.go applySchema round-trips object output through
	// any/float64. Out=any with no OutputSchema is its supported raw-output
	// path (server.go toolForErr), preserving int64 in structured and text JSON.
	mcp.AddTool(server, &mcp.Tool{
		Name: name, Description: description,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in I) (*mcp.CallToolResult, any, error) {
		out := boundMCPReply(run(svc.WithContext(ctx), in))
		return &mcp.CallToolResult{IsError: out.Error != ""}, out, nil
	})
}
