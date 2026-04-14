package lint

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yogirk/sparks/internal/manifest"
)

func TestCheckDuplicateAliases(t *testing.T) {
	pages := []manifest.PageInfo{
		{Path: "wiki/entities/A.md", Title: "A", Aliases: []string{"Shared"}},
		{Path: "wiki/entities/B.md", Title: "B", Aliases: []string{"shared"}},
		{Path: "wiki/entities/C.md", Title: "C", Aliases: []string{"Unique"}},
	}
	issues := checkDuplicateAliases(pages)
	if len(issues) != 2 {
		t.Errorf("issues = %d, want 2 (A and B both claim 'shared')", len(issues))
	}
	for _, iss := range issues {
		if iss.Check != "duplicate-aliases" {
			t.Errorf("check = %q", iss.Check)
		}
	}
}

func TestCheckFrontmatterRequiredFieldsAndEnums(t *testing.T) {
	pages := []manifest.PageInfo{
		{Path: "wiki/entities/BadType.md", Title: "BadType", Type: "bogus", Maturity: "seed", Created: "2026-04-10", Updated: "2026-04-10", Sources: []string{"x"}},
		{Path: "wiki/entities/NoTitle.md", Title: "", Type: "entity", Maturity: "seed", Created: "2026-04-10", Updated: "2026-04-10", Sources: []string{"x"}},
		{Path: "wiki/entities/BadDate.md", Title: "BadDate", Type: "entity", Maturity: "seed", Created: "not-a-date", Updated: "2026-04-10", Sources: []string{"x"}},
		{Path: "wiki/entities/Good.md", Title: "Good", Type: "entity", Maturity: "seed", Created: "2026-04-10", Updated: "2026-04-10", Sources: []string{"x"}},
	}
	issues := checkFrontmatter(pages)
	counts := map[string]int{}
	for _, iss := range issues {
		counts[iss.Check]++
	}
	if counts["invalid-frontmatter"] == 0 {
		t.Errorf("expected invalid-frontmatter, got %+v", counts)
	}
	if counts["missing-frontmatter"] == 0 {
		t.Errorf("expected missing-frontmatter, got %+v", counts)
	}
}

func TestCheckDeadSourcesFlagsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	// Create one real source file so we can distinguish present vs missing.
	_ = os.MkdirAll(filepath.Join(dir, "raw"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "raw/exists.md"), []byte("x"), 0o644)

	v := realVault(dir)
	pages := []manifest.PageInfo{{
		Path:    "wiki/entities/Thing.md",
		Title:   "Thing",
		Sources: []string{"raw/exists.md", "raw/ghost.md"},
	}}
	issues := checkDeadSources(v, pages)
	if len(issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(issues))
	}
	if issues[0].Path != "wiki/entities/Thing.md" {
		t.Errorf("path = %q", issues[0].Path)
	}
}

func TestIsStaleComparesMTime(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "raw/note.md")
	_ = os.MkdirAll(filepath.Dir(src), 0o755)
	_ = os.WriteFile(src, []byte("x"), 0o644)

	// Set source mtime to a specific past date, so staleness depends on
	// the wiki updated field relative to that.
	future := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	_ = os.Chtimes(src, future, future)

	v := realVault(dir)

	freshPage := manifest.PageInfo{
		Path:    "wiki/entities/A.md",
		Sources: []string{"raw/note.md"},
		Updated: "2026-04-10", // same day as source
	}
	if isStale(v, freshPage) {
		t.Error("same-day updated should NOT be stale (end-of-day comparison)")
	}

	stalePage := manifest.PageInfo{
		Path:    "wiki/entities/B.md",
		Sources: []string{"raw/note.md"},
		Updated: "2026-04-09",
	}
	if !isStale(v, stalePage) {
		t.Error("older updated should be stale")
	}
}

func TestCountSentences(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{"", 0},
		{"One sentence.", 1},
		{"One. Two. Three.", 3},
		{"# Heading\nNo sentences.", 1},
		{"Prose.\n```\nCode. Ignored.\n```\nMore.", 2},
		{"Question? Yes! Answer.", 3},
	}
	for _, c := range cases {
		if got := countSentences(c.body); got != c.want {
			t.Errorf("body=%q got=%d want=%d", c.body, got, c.want)
		}
	}
}

