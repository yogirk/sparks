package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
)

// pageTypeOrder is the display order for `sparks status` output. Using a
// fixed order keeps the human-facing summary stable across runs.
var pageTypeOrder = []string{"entity", "concept", "summary", "synthesis", "collection"}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Print a vault overview",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	rep, err := core.Status(v, db)
	if err != nil {
		return err
	}
	printStatus(cmd.OutOrStdout(), rep)
	return nil
}

func printStatus(out io.Writer, r core.StatusReport) {
	name := r.VaultName
	if name == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(out, "vault: %s (%s)\n", name, r.VaultRoot)
	fmt.Fprintf(out, "schema: v%d\n", r.SchemaVersion)
	fmt.Fprintf(out, "pages: %s\n", formatPageCounts(r.PagesByType))
	fmt.Fprintf(out, "inbox: %d entries pending\n", r.InboxPending)
	fmt.Fprintf(out, "manifest: %d files tracked, %d changed since last ingest, %d deleted\n",
		r.Files.Total, r.Files.Changed, r.Files.Deleted)
}

func formatPageCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "(none yet)"
	}
	seen := make(map[string]bool, len(counts))
	parts := make([]string, 0, len(counts))
	for _, t := range pageTypeOrder {
		parts = append(parts, fmt.Sprintf("%d %s", counts[t], plural(t, counts[t])))
		seen[t] = true
	}
	// Anything outside the canonical set still gets surfaced so unexpected
	// values are visible.
	var extras []string
	for t := range counts {
		if !seen[t] {
			extras = append(extras, fmt.Sprintf("%d %s", counts[t], t))
		}
	}
	sort.Strings(extras)
	parts = append(parts, extras...)
	return joinComma(parts)
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
