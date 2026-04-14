package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
)

func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Rebuild wiki/index.md from manifest state (preserves descriptions)",
		Args:  cobra.NoArgs,
		RunE:  runIndex,
	}
	cmd.Flags().Bool("dry-run", false, "report what would change, do not write")
	return cmd
}

func runIndex(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	dry, _ := cmd.Flags().GetBool("dry-run")
	res, err := core.Index(v, dry)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if dry {
		fmt.Fprintln(out, "Dry run — no files written.")
	} else {
		fmt.Fprintf(out, "Wrote %s (%d bytes).\n", res.OutputPath, res.Bytes)
	}
	for _, t := range []string{"entity", "concept", "synthesis", "summary", "collection"} {
		if n := res.PagesByType[t]; n > 0 {
			fmt.Fprintf(out, "  %s: %d\n", t, n)
		}
	}
	fmt.Fprintf(out, "Descriptions preserved: %d, blank: %d\n",
		res.DescriptionsKept, res.DescriptionsBlank)
	return nil
}
