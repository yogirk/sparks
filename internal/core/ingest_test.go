package core

import (
	"errors"
	"os"
	"sort"
	"testing"

	"github.com/yogirk/sparks/internal/manifest"
)

func TestPrepareEmptyInbox(t *testing.T) {
	v, db := freshVault(t)

	res, err := PrepareIngest(v, db)
	if err != nil {
		t.Fatalf("PrepareIngest: %v", err)
	}
	if res.Total != 0 {
		t.Errorf("Total = %d, want 0", res.Total)
	}
	// No ingest row opened for empty inbox — the lock is cheap, but opening
	// one with zero entries is noise.
	if _, err := db.CurrentIngest(); !errors.Is(err, manifest.ErrNotFound) {
		t.Errorf("empty-inbox prepare opened an ingest row: err = %v", err)
	}
}

func TestPrepareBasicEntries(t *testing.T) {
	v, db := freshVault(t)
	writeInbox(t, v.InboxPath(), `# Inbox
---
2026-04-10
First capture: https://example.com/a
- [ ] Do the thing
---
Second capture.
book: Structure and Interpretation of Computer Programs
`)

	res, err := PrepareIngest(v, db)
	if err != nil {
		t.Fatalf("PrepareIngest: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("Total = %d, want 2", res.Total)
	}
	if res.IngestID == 0 {
		t.Error("IngestID = 0, want non-zero")
	}

	e1 := res.Entries[0]
	if e1.CaptureDate != "2026-04-10" {
		t.Errorf("entry 1 CaptureDate = %q", e1.CaptureDate)
	}
	if len(e1.Hints.Tasks) != 1 || len(e1.Hints.Bookmarks) != 1 {
		t.Errorf("entry 1 hints = %+v", e1.Hints)
	}

	e2 := res.Entries[1]
	if !e2.Hints.HasBook {
		t.Errorf("entry 2 HasBook = false, want true")
	}

	// Affected collections should include Tasks, Bookmarks, Books, Ideas.
	got := res.AffectedCollections
	sort.Strings(got)
	wantContains := []string{"Books", "Bookmarks", "Ideas", "Tasks"}
	for _, w := range wantContains {
		if !contains(got, w) {
			t.Errorf("AffectedCollections = %v, missing %q", got, w)
		}
	}
}

func TestPrepareBlocksConcurrent(t *testing.T) {
	v, db := freshVault(t)
	writeInbox(t, v.InboxPath(), "# Inbox\n---\nentry\n")

	res1, err := PrepareIngest(v, db)
	if err != nil {
		t.Fatalf("first Prepare: %v", err)
	}
	res2, err := PrepareIngest(v, db)
	if !errors.Is(err, manifest.ErrIngestInProgress) {
		t.Fatalf("second Prepare err = %v, want ErrIngestInProgress", err)
	}
	if !res2.AlreadyInProgress {
		t.Error("AlreadyInProgress = false, want true")
	}
	if res2.IngestID != res1.IngestID {
		t.Errorf("second Prepare returned IngestID %d, want %d (same row)", res2.IngestID, res1.IngestID)
	}
}

func writeInbox(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
