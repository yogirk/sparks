// Package mcp exposes Sparks's core operations as MCP tools over a stdio
// JSON-RPC transport. Every handler is a thin adapter: open the vault,
// call internal/core, marshal the typed result to JSON, return.
//
// Decision A1 (thin adapters) applies here exactly as it does to the
// CLI: this file holds zero business logic. If you find yourself
// adding regex, file walks, or git calls here, you're in the wrong
// layer — push it into internal/core.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/yogirk/sparks/internal/collections"
	"github.com/yogirk/sparks/internal/contract"
	"github.com/yogirk/sparks/internal/core"
	"github.com/yogirk/sparks/internal/lint"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// Serve runs the MCP server over stdio. Blocks until the client
// disconnects. Every tool resolves the vault from the working directory
// the server was launched in — same convention as the CLI.
func Serve(version string) error {
	s := server.NewMCPServer("sparks", version)
	registerTools(s)
	return server.ServeStdio(s)
}

// registerTools is exported only so tests can register against an
// in-process server. Production callers use Serve.
func registerTools(s *server.MCPServer) {
	s.AddTool(mcpgo.NewTool("sparks_status",
		mcpgo.WithDescription("Print a vault overview (page counts by type, inbox pending, manifest stats)."),
	), handleStatus)

	s.AddTool(mcpgo.NewTool("sparks_scan",
		mcpgo.WithDescription("Scan the vault filesystem and refresh the manifest. Incremental: unchanged files are skipped."),
	), handleScan)

	s.AddTool(mcpgo.NewTool("sparks_describe",
		mcpgo.WithDescription("Return the canonical agent-runtime contract — what this vault expects from you."),
	), handleDescribe)

	s.AddTool(mcpgo.NewTool("sparks_lint",
		mcpgo.WithDescription("Run all eight deterministic lint checks against the vault. Returns issues grouped by check."),
	), handleLint)

	s.AddTool(mcpgo.NewTool("sparks_affected",
		mcpgo.WithDescription("Report which collections need regeneration since the last completed ingest."),
	), handleAffected)

	s.AddTool(mcpgo.NewTool("sparks_index",
		mcpgo.WithDescription("Rebuild wiki/index.md from manifest state, preserving agent-authored descriptions."),
	), handleIndex)

	s.AddTool(mcpgo.NewTool("sparks_prepare_ingest",
		mcpgo.WithDescription("Parse inbox.md, classify hints, open an in_progress ingest row. Returns the structured manifest of entries."),
	), handlePrepareIngest)

	s.AddTool(mcpgo.NewTool("sparks_finalize_ingest",
		mcpgo.WithDescription("Archive entries, clear the inbox header-preserving, rescan, commit. Requires a prior prepare."),
		mcpgo.WithString("message", mcpgo.Description("Optional git commit message. Defaults to 'ingest: N entries'.")),
	), handleFinalizeIngest)

	s.AddTool(mcpgo.NewTool("sparks_done",
		mcpgo.WithDescription("Mark an open task complete. Exact substring match wins; falls back to fuzzy. Ambiguous matches return candidates."),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Task text or substring to match.")),
	), handleDone)

	s.AddTool(mcpgo.NewTool("sparks_tasks_add",
		mcpgo.WithDescription("Append a `- [ ] text` line under a section heading in wiki/collections/Tasks.md. Section is created if missing."),
		mcpgo.WithString("section", mcpgo.Required(), mcpgo.Description("Section heading, typically a wikilink like '[[Sparks]]'.")),
		mcpgo.WithString("text", mcpgo.Required(), mcpgo.Description("Task text. Will be prefixed with `- [ ] `.")),
	), handleTasksAdd)

	s.AddTool(mcpgo.NewTool("sparks_query",
		mcpgo.WithDescription("Structured lookup over the manifest. Combine filters via AND. NOT semantic search."),
		mcpgo.WithString("title", mcpgo.Description("Exact title (case-insensitive).")),
		mcpgo.WithString("alias", mcpgo.Description("Match in any alias.")),
		mcpgo.WithString("tag", mcpgo.Description("Page has this tag.")),
		mcpgo.WithString("type", mcpgo.Description("entity|concept|summary|synthesis|collection")),
		mcpgo.WithString("maturity", mcpgo.Description("seed|working|stable|historical")),
		mcpgo.WithString("linked_from", mcpgo.Description("Page is linked from this path.")),
		mcpgo.WithString("links_to", mcpgo.Description("Page links to this title.")),
		mcpgo.WithBoolean("stale", mcpgo.Description("Only stale pages.")),
		mcpgo.WithBoolean("orphan", mcpgo.Description("Only orphan pages.")),
	), handleQuery)
}

// --- handlers (thin adapters) ---

func handleStatus(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, db, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	rep, err := core.Status(v, db)
	return jsonOrError(rep, err)
}

func handleScan(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, db, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	res, err := core.Scan(v, db)
	return jsonOrError(res, err)
}

func handleDescribe(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultText(contract.Markdown()), nil
}

func handleLint(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, db, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	report, err := lint.Run(v, db)
	return jsonOrError(report, err)
}

func handleAffected(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, db, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	res, err := core.Affected(v, db)
	return jsonOrError(res, err)
}

func handleIndex(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, _, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	// Index opens its own DB to keep the call site simple.
	res, err := core.Index(v, false)
	return jsonOrError(res, err)
}

func handlePrepareIngest(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, db, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	res, err := core.PrepareIngest(v, db)
	if err != nil && !res.AlreadyInProgress {
		return errorResult(err), nil
	}
	return jsonResult(res)
}

func handleFinalizeIngest(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, db, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	msg := req.GetString("message", "")
	res, err := core.FinalizeIngest(v, db, msg)
	return jsonOrError(res, err)
}

func handleDone(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, _, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	query, err := req.RequireString("query")
	if err != nil {
		return errorResult(err), nil
	}
	res, err := core.TaskDone(v, query)
	return jsonOrError(res, err)
}

func handleTasksAdd(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, _, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	section, err := req.RequireString("section")
	if err != nil {
		return errorResult(err), nil
	}
	text, err := req.RequireString("text")
	if err != nil {
		return errorResult(err), nil
	}
	res, err := core.TasksAdd(v, section, text)
	return jsonOrError(res, err)
}

func handleQuery(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	v, db, cleanup, err := openVaultDot()
	if err != nil {
		return errorResult(err), nil
	}
	defer cleanup()
	f := manifest.QueryFilter{
		Title:      req.GetString("title", ""),
		Alias:      req.GetString("alias", ""),
		Tag:        req.GetString("tag", ""),
		Type:       req.GetString("type", ""),
		Maturity:   req.GetString("maturity", ""),
		LinkedFrom: req.GetString("linked_from", ""),
		LinksTo:    req.GetString("links_to", ""),
		Stale:      req.GetBool("stale", false),
		Orphan:     req.GetBool("orphan", false),
	}
	pages, err := db.Query(vaultAdapter{v}, f)
	return jsonOrError(pages, err)
}

// --- helpers ---

// openVaultDot opens a vault rooted at the current working directory.
// Returns a cleanup func that closes the manifest. If the vault can't
// be discovered, returns an error result the caller can wrap.
func openVaultDot() (*vault.Vault, *manifest.DB, func(), error) {
	v, err := vault.Open(".")
	if err != nil {
		return nil, nil, func() {}, err
	}
	db, err := manifest.Open(v.ManifestPath())
	if err != nil {
		return nil, nil, func() {}, err
	}
	return v, db, func() { _ = db.Close() }, nil
}

// jsonOrError marshals res to JSON. On non-nil err, returns a structured
// error result so the agent gets both the partial result and the error.
func jsonOrError(res any, err error) (*mcpgo.CallToolResult, error) {
	if err != nil {
		// Embed the partial result alongside the error for richer client UX.
		body, _ := json.MarshalIndent(map[string]any{
			"error":   err.Error(),
			"partial": res,
		}, "", "  ")
		return mcpgo.NewToolResultError(string(body)), nil
	}
	return jsonResult(res)
}

func jsonResult(res any) (*mcpgo.CallToolResult, error) {
	body, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return errorResult(fmt.Errorf("marshal result: %w", err)), nil
	}
	return mcpgo.NewToolResultText(string(body)), nil
}

func errorResult(err error) *mcpgo.CallToolResult {
	return mcpgo.NewToolResultError(err.Error())
}

// vaultAdapter satisfies manifest.Vaultish without the manifest package
// needing to import vault. Mirror of the same adapter in cmd/sparks.
type vaultAdapter struct{ v *vault.Vault }

func (va vaultAdapter) RootDir() string { return va.v.Root }

// Compile-time check that we use collections.OrderedNames somewhere so
// the import doesn't go stale if/when we add a sparks_collections_regen
// tool. Removing this is safe; it just documents intent.
var _ = collections.OrderedNames
