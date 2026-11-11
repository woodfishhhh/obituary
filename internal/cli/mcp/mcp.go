package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
)

func NewCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use: "mcp", Short: "Serve Obituary tools over MCP stdio", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			return NewServer(svc, version).Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
}
