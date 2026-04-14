package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set by goreleaser via -ldflags. Default is the dev marker so
// `sparks --version` is informative even from a `go build`.
var Version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sparks:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var verbose bool
	root := &cobra.Command{
		Use:           "sparks",
		Short:         "Knowledge base runtime for AI agents",
		Long:          "Sparks maintains the mechanical integrity of an agent-driven personal knowledge base.\nSee `sparks describe` for the full agent contract.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	root.AddCommand(
		newInitCmd(),
		newScanCmd(),
		newStatusCmd(),
		newIngestCmd(),
		newDoneCmd(),
		newTasksCmd(),
		newLintCmd(),
		newFmtCmd(),
		newCollectionsCmd(),
		newIndexCmd(),
		newQueryCmd(),
		newAffectedCmd(),
		newDescribeCmd(),
		newServeCmd(),
	)
	return root
}
