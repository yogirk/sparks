package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
)

func newAffectedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "affected",
		Short: "Report which collections need regeneration since last ingest",
		Args:  cobra.NoArgs,
		RunE:  runAffected,
	}
	cmd.Flags().Bool("json", false, "machine-readable JSON output")
	return cmd
}

func runAffected(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	res, err := core.Affected(v, db)
	if err != nil {
		return err
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	out := cmd.OutOrStdout()
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	if len(res.Affected) == 0 {
		fmt.Fprintln(out, "none")
		return nil
	}
	fmt.Fprintln(out, strings.Join(res.Affected, ","))
	return nil
}
