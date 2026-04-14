package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/collections"
)

func newCollectionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collections",
		Short: "Manage auto-generated collection pages under wiki/collections/",
	}
	cmd.AddCommand(newCollectionsRegenCmd())
	return cmd
}

func newCollectionsRegenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "regen [name...]",
		Short: "Regenerate one or more collections (Tasks is excluded — it's live)",
		Long: `Regenerates collection pages from raw/ files (and the manifest, for
Projects). Without arguments, regenerates every collection except Tasks.
With names (e.g. "Quotes Bookmarks"), regenerates only those.

Use --dry-run to preview what would change without writing.`,
		Args: cobra.ArbitraryArgs,
		RunE: runCollectionsRegen,
	}
	cmd.Flags().Bool("dry-run", false, "report what would change, no writes")
	cmd.Flags().Bool("json", false, "machine-readable JSON output")
	cmd.Flags().Bool("all", false, "explicit no-op for clarity (default behavior)")
	return cmd
}

func runCollectionsRegen(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()

	dry, _ := cmd.Flags().GetBool("dry-run")
	results, err := collections.Regenerate(v, db, collections.RegenerateOptions{
		Names:  args,
		DryRun: dry,
	})
	if err != nil {
		return err
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		return writeCollectionsJSON(cmd.OutOrStdout(), results)
	}
	writeCollectionsText(cmd.OutOrStdout(), results, dry)
	return nil
}

func writeCollectionsJSON(out io.Writer, results []collections.Result) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func writeCollectionsText(out io.Writer, results []collections.Result, dry bool) {
	if dry {
		fmt.Fprintln(out, "Dry run — no files were written.")
	}
	for _, r := range results {
		switch {
		case r.Error != "":
			fmt.Fprintf(out, "  [ERR ] %s — %s\n", r.Name, r.Error)
		case r.Skipped:
			fmt.Fprintf(out, "  [skip] %s — %s\n", r.Name, r.SkipReason)
		default:
			fmt.Fprintf(out, "  [ ok ] %s → %s (%d bytes)\n", r.Name, r.OutputPath, r.Bytes)
		}
	}
	fmt.Fprintf(out, "%d collection(s) processed.\n", len(results))
}
