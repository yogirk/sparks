package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
)

func newBriefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Emit a structured snapshot of recent vault activity for agent synthesis",
		Long: `Gathers the plumbing an agent needs to write a weekly brief:
log entries, new raw captures, wiki pages updated in the window,
pages worth revisiting (stale / seed-orphan / thin), and an open-task
count. Synthesis itself — what to highlight, what to connect — is
agent work.

Default window is the last 7 calendar days. Human output is compact;
` + "`--json`" + ` gives the agent the full BriefReport.`,
		Args: cobra.NoArgs,
		RunE: runBrief,
	}
	cmd.Flags().Int("days", core.DefaultBriefDays, "window size in calendar days")
	cmd.Flags().Bool("json", false, "emit BriefReport as JSON")
	return cmd
}

func runBrief(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	days, _ := cmd.Flags().GetInt("days")
	rep, err := core.Brief(v, db, days)
	if err != nil {
		return err
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	printBrief(cmd.OutOrStdout(), rep)
	return nil
}

func printBrief(out io.Writer, r core.BriefReport) {
	fmt.Fprintf(out, "brief: last %d days (since %s)\n", r.Window.Days, r.Window.Since.Format("2006-01-02"))
	fmt.Fprintf(out, "  log entries: %d\n", len(r.LogEntries))
	fmt.Fprintf(out, "  new raw:     %d\n", len(r.NewRaw))
	fmt.Fprintf(out, "  updated wiki: %d\n", len(r.UpdatedWiki))
	fmt.Fprintf(out, "  revisit:     %d stale, %d seed-orphans, %d thin\n",
		len(r.Revisit.Stale), len(r.Revisit.SeedOrphans), len(r.Revisit.Thin))
	fmt.Fprintf(out, "  open tasks:  %d\n", r.Tasks.Open)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Pipe --json to your agent; it will synthesize the brief.")
}
