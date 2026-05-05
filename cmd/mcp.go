package cmd

import (
	"github.com/CosmoLabs-org/SmokeSig/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for Claude Desktop integration",
	Long: `Start an MCP (Model Context Protocol) server that exposes smoke test
operations as tools. Designed for use with Claude Desktop and other MCP clients.

Add to your Claude Desktop configuration:
  {
    "mcpServers": {
      "SmokeSig": {
        "command": "smoke",
        "args": ["mcp"]
      }
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := mcp.NewServer()
		return srv.ServeStdio()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
