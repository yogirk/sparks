package collections

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

func TestRegenerateAllOnEmptyVaultProducesAllPagesWithEmptyMarkers(t *testing.T) {
	v, db := freshVault(t)

	results, err := Regenerate(v, db, RegenerateOptions{})
	if err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	if len(results) != len(OrderedNames) {
		t.Errorf("results = %d, want %d", len(results), len(OrderedNames))
	}
	for _, r := range results {
		if r.Error != "" {
			t.Errorf("collection %s errored: %s", r.Name, r.Error)
		}
		body := readVaultFile(t, v, "wiki/collections/"+OutputFilename[r.Name])
		if !strings.Contains(body, "AUTO-GENERATED") {
			t.Errorf("collection %s missing AUTO-GENERATED banner:\n%s", r.Name, body)
		}
	}
}

func TestRegenerateNamedSubsetOnly(t *testing.T) {
	v, db := freshVault(t)

	results, err := Regenerate(v, db, RegenerateOptions{Names: []string{"Quotes", "Ideas"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("results = %d, want 2", len(results))
	}
	// Only the selected files should exist on disk.
	for _, name := range []string{"Bookmarks.md", "Books.md", "Reading List.md", "Media.md", "Projects.md"} {
		if _, err := os.Stat(filepath.Join(v.Root, "wiki", "collections", name)); !os.IsNotExist(err) {
			t.Errorf("%s should not exist after named regen", name)
		}
	}
}

func TestExtractBookmarksWithFixtures(t *testing.T) {
	v, _ := freshVault(t)
	writeFixture(t, v.Root, "raw/weblinks/2026-04.md", `# Weblinks April

- https://example.com/a Cool article on widgets
- https://example.com/b
plain text without urls
- See https://news.ycombinator.com/item?id=12345 for context.
`)

	body, err := extractBookmarks(v, nil, DefaultGlobs["Bookmarks"])
	if err != nil {
		t.Fatalf("extractBookmarks: %v", err)
	}
	if !strings.Contains(body, "https://example.com/a") {
		t.Errorf("missing example.com/a: %s", body)
	}
	if !strings.Contains(body, "https://news.ycombinator.com/item?id=12345") {
		t.Errorf("missing HN URL with query string: %s", body)
	}
	if !strings.Contains(body, "3 bookmarks") {
		t.Errorf("expected 3 bookmarks header: %s", body)
	}
}

func TestExtractQuotesParsesMultilineAndAttribution(t *testing.T) {
	v, _ := freshVault(t)
	writeFixture(t, v.Root, "raw/quotes/2026.md", `> The best code is no code.
— DHH

> Programs must be written for people to read,
> and only incidentally for machines to execute.
— Hal Abelson, SICP

> An orphan quote without attribution.

`)
	body, err := extractQuotes(v, nil, DefaultGlobs["Quotes"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "DHH") {
		t.Error("missing DHH attribution")
	}
	if !strings.Contains(body, "Hal Abelson, SICP") {
		t.Error("missing SICP attribution")
	}
	if !strings.Contains(body, "Programs must be written") {
		t.Error("multiline quote not preserved")
	}
	if !strings.Contains(body, "(no attribution)") {
		t.Error("orphan quote should be marked '(no attribution)'")
	}
	if !strings.Contains(body, "3 quotes") {
		t.Errorf("expected 3 quotes count: %s", body)
	}
}

func TestExtractBooksFromQuotesAndInbox(t *testing.T) {
	v, _ := freshVault(t)
	writeFixture(t, v.Root, "raw/quotes/q.md", `> A small thing.
— Tracy Kidder, The Soul of a New Machine
`)
	writeFixture(t, v.Root, "raw/inbox/2026-04-10.md", `---
book: Designing Data-Intensive Applications / Kleppmann
`)
	body, err := extractBooks(v, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "The Soul of a New Machine") {
		t.Error("book derived from quote attribution missing")
	}
	if !strings.Contains(body, "Designing Data-Intensive Applications / Kleppmann") {
		t.Error("book from inbox hint missing")
	}
}

func TestExtractReadingListFromInbox(t *testing.T) {
	v, _ := freshVault(t)
	writeFixture(t, v.Root, "raw/inbox/2026-04-10.md", `---
to-read: https://paulgraham.com/greatwork.html
to-read: Karpathy on tokens
`)
	body, err := extractReadingList(v, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "greatwork.html") || !strings.Contains(body, "Karpathy on tokens") {
		t.Errorf("missing entries:\n%s", body)
	}
}

func TestExtractIdeasGroupsByStatus(t *testing.T) {
	v, _ := freshVault(t)
	writeFixture(t, v.Root, "raw/ideas/sparks.md", `# Sparks
status: shipping
A KB runtime.
`)
	writeFixture(t, v.Root, "raw/ideas/parked.md", `# Parked One
status: parked
Maybe later.
`)
	writeFixture(t, v.Root, "raw/ideas/loose.md", `# No Status
Just an idea.
`)
	body, err := extractIdeas(v, nil, DefaultGlobs["Ideas"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "## Shipping") {
		t.Error("missing shipping group")
	}
	if !strings.Contains(body, "## Parked") {
		t.Error("missing parked group")
	}
	if !strings.Contains(body, "## Uncategorized") {
		t.Error("missing uncategorized group for status-less idea")
	}
}

func TestRegenerateDryRunDoesNotWrite(t *testing.T) {
	v, db := freshVault(t)
	results, err := Regenerate(v, db, RegenerateOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	for _, r := range results {
		if _, err := os.Stat(r.OutputPath); err == nil {
			t.Errorf("dry-run wrote %s", r.OutputPath)
		}
	}
}

// freshVault creates a brand-new vault and returns vault + manifest.
func freshVault(t *testing.T) (*vault.Vault, *manifest.DB) {
	t.Helper()
	dir := t.TempDir()
	if _, err := vault.Init(vault.InitOptions{Path: dir}); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db, err := manifest.Open(v.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return v, db
}

func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readVaultFile(t *testing.T, v *vault.Vault, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(v.Root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}
