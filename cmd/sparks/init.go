package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a Sparks vault (idempotent)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runInit,
	}
	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	res, err := core.InitVault(path)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if res.Existed {
		fmt.Fprintf(out, "Vault already exists at %s — repaired any missing dirs/inbox.\n", res.VaultRoot)
	} else {
		fmt.Fprintf(out, "Initialized Sparks vault at %s\n", res.VaultRoot)
	}
	if res.ManifestNew {
		fmt.Fprintln(out, "  Created sparks.db (schema v1)")
	}
	return nil
}
