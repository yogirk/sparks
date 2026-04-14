package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
	"github.com/yogirk/sparks/internal/inbox"
	"github.com/yogirk/sparks/internal/manifest"
)

func newIngestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Process inbox.md (two-phase: --prepare, then --finalize)",
		Long: `Two-phase ingest.

--prepare parses inbox.md, returns a structured manifest of entries with
deterministic hints (URLs, task markers, book/to-read prefixes, quote
markers). The agent reads the output, creates or updates wiki pages, then
calls --finalize to archive the entries and commit.`,
		Args: cobra.NoArgs,
		RunE: runIngest,
	}
	cmd.Flags().Bool("prepare", false, "parse inbox and emit a structured manifest for the agent")
	cmd.Flags().Bool("finalize", false, "archive entries, clear inbox, scan, commit (after agent work)")
	cmd.Flags().Bool("abort", false, "abort an in_progress ingest to unblock the next --prepare")
	cmd.Flags().Bool("json", false, "machine-readable JSON output (default for --prepare)")
	cmd.Flags().StringP("message", "m", "", "git commit message (used with --finalize)")
	return cmd
}

func runIngest(cmd *cobra.Command, args []string) error {
	prepare, _ := cmd.Flags().GetBool("prepare")
	finalize, _ := cmd.Flags().GetBool("finalize")
	abort, _ := cmd.Flags().GetBool("abort")

	n := boolN(prepare) + boolN(finalize) + boolN(abort)
	if n != 1 {
		return fmt.Errorf("specify exactly one of --prepare, --finalize, --abort")
	}
	if finalize {
		return runFinalize(cmd)
	}
	if abort {
		return runAbort(cmd)
	}
	return runPrepare(cmd)
}

func runFinalize(cmd *cobra.Command) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	msg, _ := cmd.Flags().GetString("message")
	res, err := core.FinalizeIngest(v, db, msg)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Finalized ingest %d: %d entries archived across %d date file(s).\n",
		res.IngestID, res.EntryCount, res.ArchivedFiles)
	if res.CommitSHA != "" {
		fmt.Fprintf(out, "Committed %s.\n", res.CommitSHA)
	} else if res.CommitSkipped != "" {
		fmt.Fprintf(out, "Commit skipped: %s\n", res.CommitSkipped)
	}
	return nil
}

func boolN(b bool) int {
	if b {
		return 1
	}
	return 0
}

func runAbort(cmd *cobra.Command) error {
	_, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	current, err := db.CurrentIngest()
	if errors.Is(err, manifest.ErrNotFound) {
		fmt.Fprintln(cmd.OutOrStdout(), "No ingest in progress.")
		return nil
	}
	if err != nil {
		return err
	}
	if err := db.AbortIngest(current.ID); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Aborted ingest %d (started %s).\n",
		current.ID, current.StartedAt.Format("2006-01-02 15:04:05"))
	return nil
}

func runPrepare(cmd *cobra.Command) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()

	res, err := core.PrepareIngest(v, db)
	if err != nil && !errors.Is(err, manifest.ErrIngestInProgress) {
		return err
	}

	asJSON, _ := cmd.Flags().GetBool("json")
	// --prepare defaults to JSON because the agent consumes it. Override
	// with explicit --json=false if a human wants the text summary.
	if !cmd.Flags().Changed("json") {
		asJSON = true
	}
	out := cmd.OutOrStdout()
	if asJSON {
		return writeJSON(out, res)
	}
	return writePrepareText(out, res)
}

func writeJSON(out io.Writer, res core.PrepareResult) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func writePrepareText(out io.Writer, res core.PrepareResult) error {
	if res.AlreadyInProgress {
		fmt.Fprintf(out, "Ingest %d is already in progress. Finalize or abort it before starting a new one.\n", res.IngestID)
		return nil
	}
	if res.Total == 0 {
		fmt.Fprintln(out, "Inbox is empty (no entries after the header separator).")
		return nil
	}
	fmt.Fprintf(out, "Ingest %d opened. %d entries pending.\n", res.IngestID, res.Total)
	for _, e := range res.Entries {
		fmt.Fprintf(out, "  [#%d %s] hash=%s hints=%s\n",
			e.Index, e.CaptureDate, e.Hash[:8], hintSummary(e.Hints))
	}
	fmt.Fprintf(out, "Affected collections: %v\n", res.AffectedCollections)
	return nil
}

// hintSummary formats an entry's hints as a short comma-separated tag
// list for the text-mode summary. Empty hints render as "-".
func hintSummary(h inbox.Hints) string {
	var parts []string
	if n := len(h.Tasks); n > 0 {
		parts = append(parts, fmt.Sprintf("tasks:%d", n))
	}
	if n := len(h.Bookmarks); n > 0 {
		parts = append(parts, fmt.Sprintf("bookmarks:%d", n))
	}
	if h.HasQuote {
		parts = append(parts, "quote")
	}
	if h.HasBook {
		parts = append(parts, "book")
	}
	if h.HasToRead {
		parts = append(parts, "to-read")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}
