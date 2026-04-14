package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/lint"
)

func newLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Analyze vault health (orphans, broken links, thin pages, etc.)",
		Long: `Runs eight deterministic checks over the manifest:
orphans, broken-links, missing-frontmatter, invalid-frontmatter,
thin-pages, stale-pages, dead-sources, duplicate-aliases.

Scan the vault first (` + "`sparks scan`" + `) so the manifest reflects current state.`,
		Args: cobra.NoArgs,
		RunE: runLint,
	}
	cmd.Flags().Bool("json", false, "machine-readable JSON output")
	return cmd
}

func runLint(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	report, err := lint.Run(v, db)
	if err != nil {
		return err
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
		return writeLintJSON(cmd.OutOrStdout(), report)
	}
	writeLintText(cmd.OutOrStdout(), report)
	if len(report.Issues) > 0 {
		// Non-zero exit so lint fits into CI/hooks cleanly. Cobra wraps
		// this as a returned error; SilenceUsage on root keeps help out.
		return fmt.Errorf("%d lint issue(s) found", len(report.Issues))
	}
	return nil
}

func writeLintJSON(out io.Writer, r lint.Report) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func writeLintText(out io.Writer, r lint.Report) {
	if len(r.Issues) == 0 {
		fmt.Fprintln(out, "Vault health: OK (no lint issues).")
		return
	}
	// Group by check for a tidy summary.
	byCheck := map[string][]lint.Issue{}
	for _, iss := range r.Issues {
		byCheck[iss.Check] = append(byCheck[iss.Check], iss)
	}
	checks := make([]string, 0, len(byCheck))
	for c := range byCheck {
		checks = append(checks, c)
	}
	sort.Strings(checks)
	for _, c := range checks {
		fmt.Fprintf(out, "%s (%d):\n", c, len(byCheck[c]))
		for _, iss := range byCheck[c] {
			if iss.Path != "" {
				fmt.Fprintf(out, "  %s — %s\n", iss.Path, iss.Message)
			} else {
				fmt.Fprintf(out, "  %s\n", iss.Message)
			}
		}
	}
	fmt.Fprintf(out, "\n%d issue(s) across %d check(s).\n", len(r.Issues), len(byCheck))
}
