package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
)

func newDoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done <query>",
		Short: "Mark an open task complete (fuzzy match)",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runDone,
	}
	return cmd
}

func runDone(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	query := joinArgs(args)
	res, err := core.TaskDone(v, query)
	out := cmd.OutOrStdout()
	if errors.Is(err, core.ErrTaskAmbiguous) {
		fmt.Fprintln(out, "Ambiguous — multiple open tasks match:")
		for _, c := range res.Candidates {
			fmt.Fprintf(out, "  - %s\n", c)
		}
		return err
	}
	if errors.Is(err, core.ErrTaskNotFound) {
		fmt.Fprintf(out, "No open task matching %q.\n", query)
		return err
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Done: %s (line %d)\n", res.Matched, res.Line)
	return nil
}

// joinArgs is a tiny helper; strings.Join pulled in explicitly elsewhere.
func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
