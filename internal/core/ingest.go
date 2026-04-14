package core

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/yogirk/sparks/internal/inbox"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// PrepareResult is what `sparks ingest --prepare` returns. Agents read
// this, create/update wiki pages for each entry, then call FinalizeIngest.
type PrepareResult struct {
	IngestID             int64         `json:"ingest_id"`
	Entries              []inbox.Entry `json:"entries"`
	Total                int           `json:"total"`
	AffectedCollections  []string      `json:"affected_collections"`
	AlreadyInProgress    bool          `json:"already_in_progress,omitempty"`
}

// PrepareIngest parses inbox.md, records an in_progress ingest row, and
// returns the structured entry list. The caller is expected to call
// FinalizeIngest or AbortIngest to release the lock — leaving an ingest
// in_progress will block the next `sparks ingest --prepare`.
//
// An empty inbox is a non-error: returns PrepareResult with zero entries
// and does NOT open an in_progress row. Nothing to lock over.
func PrepareIngest(v *vault.Vault, db *manifest.DB) (PrepareResult, error) {
	data, err := os.ReadFile(v.InboxPath())
	if err != nil {
		return PrepareResult{}, fmt.Errorf("read inbox: %w", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	entries := inbox.Split(string(data), inbox.SplitOptions{Today: today})
	if len(entries) == 0 {
		return PrepareResult{Total: 0, AffectedCollections: []string{}}, nil
	}

	rec, err := db.OpenIngest()
	if errors.Is(err, manifest.ErrIngestInProgress) {
		return PrepareResult{
			IngestID:          rec.ID,
			AlreadyInProgress: true,
		}, err
	}
	if err != nil {
		return PrepareResult{}, err
	}

	return PrepareResult{
		IngestID:            rec.ID,
		Entries:             entries,
		Total:               len(entries),
		AffectedCollections: inferAffectedCollections(entries),
	}, nil
}

// inferAffectedCollections returns the collection names that will need
// regeneration after this batch of entries lands. Deterministic: look at
// hint shapes across all entries. Collections whose source globs aren't
// touched by inbox processing (Quotes, Media) are only affected if the
// agent drops files in the right raw/ subtrees — which the agent decides,
// not us. For V1, return the conservative set: collections inbox hints
// can drive directly.
func inferAffectedCollections(entries []inbox.Entry) []string {
	affected := map[string]bool{}
	for _, e := range entries {
		if len(e.Hints.Tasks) > 0 {
			affected["Tasks"] = true
		}
		if len(e.Hints.Bookmarks) > 0 {
			affected["Bookmarks"] = true
		}
		if e.Hints.HasBook {
			affected["Books"] = true
		}
		if e.Hints.HasToRead {
			affected["ReadingList"] = true
		}
		if e.Hints.HasQuote {
			affected["Quotes"] = true
		}
	}
	// Ideas is always a candidate for inbox processing — entries without
	// any hint often turn into Ideas pages. Keep it conservative and
	// include Ideas whenever there's at least one entry.
	if len(entries) > 0 {
		affected["Ideas"] = true
	}

	out := make([]string, 0, len(affected))
	for k := range affected {
		out = append(out, k)
	}
	return sortStrings(out)
}

// sortStrings is inline to avoid pulling in sort for one call site. Pure
// O(n²) insertion sort, which is fine for N <= 8 collections.
func sortStrings(in []string) []string {
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && in[j-1] > in[j]; j-- {
			in[j-1], in[j] = in[j], in[j-1]
		}
	}
	return in
}
