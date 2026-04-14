package core

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/yogirk/sparks/internal/git"
	"github.com/yogirk/sparks/internal/inbox"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// FinalizeResult summarizes what `sparks ingest --finalize` did.
type FinalizeResult struct {
	IngestID       int64  `json:"ingest_id"`
	EntryCount     int    `json:"entry_count"`
	ArchivedFiles  int    `json:"archived_files"`
	InboxBytesKept int    `json:"inbox_bytes_kept"`
	BytesRemoved   int    `json:"bytes_removed"`
	CommitSHA      string `json:"commit_sha,omitempty"`
	CommitSkipped  string `json:"commit_skipped,omitempty"` // reason if commit didn't happen
}

// FinalizeIngest archives pending inbox entries, clears the inbox, rescans
// the manifest, and (optionally) commits the result via the vault's git
// identity. Requires a prior successful PrepareIngest.
//
// Ordering (archive → clear → scan → commit → complete row) is chosen so
// that a crash anywhere leaves the vault in a recoverable state:
//
//   - Crash before clear: inbox still holds entries, archive has the
//     same entries appended. User aborts + reprepares.
//   - Crash after clear, before commit: working tree dirty. User
//     inspects and commits manually.
//   - Crash after commit, before row complete: ingest row stays
//     in_progress. User runs --abort, no data loss.
//
// Message defaults to "ingest: N entries" if empty. auto_commit=false in
// sparks.toml skips the commit step entirely.
func FinalizeIngest(v *vault.Vault, db *manifest.DB, message string) (FinalizeResult, error) {
	current, err := db.CurrentIngest()
	if errors.Is(err, manifest.ErrNotFound) {
		return FinalizeResult{}, fmt.Errorf("no in_progress ingest: run `sparks ingest --prepare` first")
	}
	if err != nil {
		return FinalizeResult{}, err
	}

	data, err := os.ReadFile(v.InboxPath())
	if err != nil {
		return FinalizeResult{}, fmt.Errorf("read inbox: %w", err)
	}
	today := time.Now().UTC().Format("2006-01-02")
	entries := inbox.Split(string(data), inbox.SplitOptions{Today: today})

	result := FinalizeResult{
		IngestID:   current.ID,
		EntryCount: len(entries),
	}

	// Archive only if there's something to archive. A --finalize on an
	// already-emptied inbox (agent cleared it manually, unusual) still
	// completes the row — the ingest conceptually did nothing.
	if len(entries) > 0 {
		if err := inbox.Archive(v.Root, entries); err != nil {
			return result, err
		}
		result.ArchivedFiles = countDistinctDates(entries)

		removed, err := inbox.ClearInbox(v.InboxPath())
		if err != nil {
			return result, err
		}
		result.BytesRemoved = removed
	}

	// Rescan so the manifest reflects the archived files + cleared inbox.
	if _, err := Scan(v, db); err != nil {
		return result, fmt.Errorf("post-finalize scan: %w", err)
	}

	// Commit is optional per vault config. auto_commit=false means the
	// human or some outer workflow drives commits.
	if v.Config.Git.AutoCommit {
		msg := message
		if msg == "" {
			msg = fmt.Sprintf("ingest: %d entries", result.EntryCount)
		}
		sha, err := git.CommitIfDirty(v.Root, msg)
		if err != nil {
			if errors.Is(err, git.ErrNotRepo) {
				result.CommitSkipped = "vault is not a git repository (run `git init` to enable auto-commit)"
			} else {
				return result, fmt.Errorf("commit: %w", err)
			}
		} else {
			result.CommitSHA = sha
			if sha == "" {
				result.CommitSkipped = "nothing to commit (working tree clean)"
			}
		}
	} else {
		result.CommitSkipped = "auto_commit = false in sparks.toml"
	}

	if err := db.CompleteIngest(current.ID, result.EntryCount, result.CommitSHA); err != nil {
		return result, fmt.Errorf("complete ingest: %w", err)
	}
	return result, nil
}

// countDistinctDates returns how many date buckets the entries span. Used
// by the summary so the human knows how many archive files got touched.
func countDistinctDates(entries []inbox.Entry) int {
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.CaptureDate] = true
	}
	return len(seen)
}
