package main

import (
	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/mcp"
)

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run as an MCP server over stdio",
		Long: `Starts an MCP (Model Context Protocol) server over stdin/stdout
JSON-RPC. Designed to be spawned by an MCP-capable harness (Claude
Code, Codex CLI, Gemini CLI, etc.).

Available tools:
  sparks_status, sparks_scan, sparks_describe,
  sparks_lint, sparks_affected, sparks_index,
  sparks_prepare_ingest, sparks_finalize_ingest,
  sparks_done, sparks_tasks_add, sparks_query

Vault is resolved from the working directory the server is launched in.`,
		Args: cobra.NoArgs,
		RunE: runServe,
	}
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	return mcp.Serve(Version)
}
