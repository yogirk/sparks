package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

func newQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Structured lookup over the manifest (NOT semantic search)",
		Long: `Filter pages by frontmatter or link-graph properties. Combine flags
with implicit AND. Output is one path per line by default; --json gives
the full PageInfo per match.`,
		Args: cobra.NoArgs,
		RunE: runQuery,
	}
	cmd.Flags().String("title", "", "exact title match (case-insensitive)")
	cmd.Flags().String("alias", "", "match in any alias (case-insensitive)")
	cmd.Flags().String("tag", "", "page has this tag")
	cmd.Flags().String("type", "", "entity|concept|summary|synthesis|collection")
	cmd.Flags().String("maturity", "", "seed|working|stable|historical")
	cmd.Flags().String("linked-from", "", "page is linked to from this path")
	cmd.Flags().String("links-to", "", "page links to this title")
	cmd.Flags().Bool("stale", false, "only stale pages")
	cmd.Flags().Bool("orphan", false, "only orphan pages (no incoming links)")
	cmd.Flags().Bool("json", false, "emit full PageInfo as JSON array")
	return cmd
}

func runQuery(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	f := manifest.QueryFilter{}
	f.Title, _ = cmd.Flags().GetString("title")
	f.Alias, _ = cmd.Flags().GetString("alias")
	f.Tag, _ = cmd.Flags().GetString("tag")
	f.Type, _ = cmd.Flags().GetString("type")
	f.Maturity, _ = cmd.Flags().GetString("maturity")
	f.LinkedFrom, _ = cmd.Flags().GetString("linked-from")
	f.LinksTo, _ = cmd.Flags().GetString("links-to")
	f.Stale, _ = cmd.Flags().GetBool("stale")
	f.Orphan, _ = cmd.Flags().GetBool("orphan")

	pages, err := db.Query(vaultAdapter{v}, f)
	if err != nil {
		return err
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		return writeQueryJSON(cmd.OutOrStdout(), pages)
	}
	for _, p := range pages {
		fmt.Fprintln(cmd.OutOrStdout(), p.Path)
	}
	return nil
}

func writeQueryJSON(out io.Writer, pages []manifest.PageInfo) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(pages)
}

// vaultAdapter satisfies manifest.Vaultish without making manifest
// import vault directly.
type vaultAdapter struct{ v *vault.Vault }

func (va vaultAdapter) RootDir() string { return va.v.Root }
