package main

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Update the SQLite manifest from the vault filesystem",
		Args:  cobra.NoArgs,
		RunE:  runScan,
	}
	return cmd
}

func runScan(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	res, err := core.Scan(v, db)
	if err != nil {
		return err
	}
	verbose, _ := cmd.Flags().GetBool("verbose")
	printScan(cmd.OutOrStdout(), res, verbose)
	return nil
}

func printScan(out io.Writer, res core.ScanResult, verbose bool) {
	fmt.Fprintf(out, "Scanned %s in %s\n", res.VaultRoot, res.Duration.Round(time.Millisecond))
	fmt.Fprintf(out, "  walked: %d  hashed: %d  skipped: %d  deleted: %d\n",
		res.Walked, res.Hashed, res.Skipped, res.Deleted)
	if len(res.Errors) > 0 {
		fmt.Fprintf(out, "  errors: %d\n", len(res.Errors))
		if verbose {
			for _, e := range res.Errors {
				fmt.Fprintf(out, "    %s: %s\n", e.Path, e.Err)
			}
		}
	}
}

// openVault is the adapter helper used by every command that needs both a
// vault and a manifest. Keeping it in cmd/ avoids leaking presentation
// concerns into internal/core.
func openVault(path string) (*vault.Vault, *manifest.DB, error) {
	v, err := vault.Open(path)
	if err != nil {
		return nil, nil, err
	}
	db, err := manifest.Open(v.ManifestPath())
	if err != nil {
		return nil, nil, err
	}
	return v, db, nil
}
