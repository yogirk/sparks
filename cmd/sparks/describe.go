package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/contract"
)

func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Print the canonical agent-runtime contract embedded in this binary",
		Long: `Prints the same contract that ` + "`sparks init --agent X`" + ` writes
into instruction files. Pipe to a file, redirect into a CLAUDE.md,
or just read it.

The contract is the source of truth for how agents operate a Sparks
vault: page types, frontmatter schema, ingest protocol, ownership
boundaries, and what NOT to do.`,
		Args: cobra.NoArgs,
		RunE: runDescribe,
	}
	return cmd
}

func runDescribe(cmd *cobra.Command, args []string) error {
	_, err := fmt.Fprint(cmd.OutOrStdout(), contract.Markdown())
	return err
}
