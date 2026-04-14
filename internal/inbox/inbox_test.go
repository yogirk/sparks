package inbox

import (
	"strings"
	"testing"
)

func TestSplitHeaderOnly(t *testing.T) {
	in := "# Inbox\n\nheader text, no separator, no entries.\n"
	got := Split(in, SplitOptions{Today: "2026-04-14"})
	if got != nil {
		t.Errorf("header-only inbox returned %d entries, want nil", len(got))
	}
}

func TestSplitSingleEntryAfterHeader(t *testing.T) {
	in := `# Inbox

header.

---

Real capture line 1.
Real capture line 2.
`
	got := Split(in, SplitOptions{Today: "2026-04-14"})
	if len(got) != 1 {
		t.Fatalf("entries = %d, want 1", len(got))
	}
	if got[0].CaptureDate != "2026-04-14" {
		t.Errorf("CaptureDate = %q, want 2026-04-14 (from Today)", got[0].CaptureDate)
	}
	if !strings.Contains(got[0].Content, "capture line 1") {
		t.Errorf("Content = %q", got[0].Content)
	}
	if got[0].Index != 1 {
		t.Errorf("Index = %d, want 1", got[0].Index)
	}
	if len(got[0].Hash) != 64 {
		t.Errorf("Hash len = %d, want 64 (sha256 hex)", len(got[0].Hash))
	}
}

func TestSplitMultipleEntriesWithCaptureDate(t *testing.T) {
	in := `# Inbox
---
2026-04-10
First entry content.
---
Second entry, no date.
---

2026-04-12

Third entry, date and blank line before content.
`
	got := Split(in, SplitOptions{Today: "2026-04-14"})
	if len(got) != 3 {
		t.Fatalf("entries = %d, want 3", len(got))
	}
	if got[0].CaptureDate != "2026-04-10" {
		t.Errorf("entry 1 CaptureDate = %q, want 2026-04-10", got[0].CaptureDate)
	}
	if got[1].CaptureDate != "2026-04-14" {
		t.Errorf("entry 2 CaptureDate = %q, want today (2026-04-14)", got[1].CaptureDate)
	}
	if got[2].CaptureDate != "2026-04-12" {
		t.Errorf("entry 3 CaptureDate = %q, want 2026-04-12", got[2].CaptureDate)
	}
	// Indices are stable and 1-based.
	for i, e := range got {
		if e.Index != i+1 {
			t.Errorf("entry %d Index = %d", i, e.Index)
		}
	}
}

func TestSplitFiltersBlankEntries(t *testing.T) {
	in := `header
---


---
Real entry.
---

---
`
	got := Split(in, SplitOptions{Today: "2026-04-14"})
	if len(got) != 1 {
		t.Fatalf("entries = %d, want 1 (whitespace-only chunks must be filtered)", len(got))
	}
}

func TestSplitIdenticalContentProducesIdenticalHash(t *testing.T) {
	in1 := "hdr\n---\nsame content\n"
	in2 := "---\nsame content\n"
	g1 := Split(in1, SplitOptions{Today: "2026-04-14"})
	g2 := Split(in2, SplitOptions{Today: "2026-04-14"})
	if len(g1) != 1 || len(g2) != 1 {
		t.Fatalf("unexpected counts g1=%d g2=%d", len(g1), len(g2))
	}
	if g1[0].Hash != g2[0].Hash {
		t.Errorf("same content → different hashes: %q vs %q", g1[0].Hash, g2[0].Hash)
	}
}

func TestClassifyURLs(t *testing.T) {
	body := `Check out https://example.com/path and https://another.org.
Duplicate: https://example.com/path (with trailing .)
`
	h := classify(body)
	if len(h.Bookmarks) != 2 {
		t.Errorf("Bookmarks = %v, want 2 unique URLs (dedupe required)", h.Bookmarks)
	}
	if h.Bookmarks[0] != "https://example.com/path" {
		t.Errorf("first bookmark = %q", h.Bookmarks[0])
	}
}

func TestClassifyTasks(t *testing.T) {
	body := `- [ ] Ship the thing
TODO: write tests
* [ ] another task
- [x] done one (should NOT be picked up)
`
	h := classify(body)
	if len(h.Tasks) != 3 {
		t.Errorf("Tasks = %v, want 3 (done tasks excluded)", h.Tasks)
	}
}

func TestClassifyFlags(t *testing.T) {
	body := `book: The Soul of a New Machine / Tracy Kidder
to-read: https://www.paulgraham.com/greatwork.html
> This is a quote line.
"Another quote" — Author
`
	h := classify(body)
	if !h.HasBook {
		t.Error("HasBook = false, want true")
	}
	if !h.HasToRead {
		t.Error("HasToRead = false, want true")
	}
	if !h.HasQuote {
		t.Error("HasQuote = false, want true")
	}
}

func TestClassifyEmpty(t *testing.T) {
	h := classify("")
	if h.HasBook || h.HasToRead || h.HasQuote {
		t.Errorf("empty body produced hints: %+v", h)
	}
	if len(h.Tasks) != 0 || len(h.Bookmarks) != 0 {
		t.Errorf("empty body produced Tasks/Bookmarks: %+v", h)
	}
}
