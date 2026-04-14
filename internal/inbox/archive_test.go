package inbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveGroupsByCaptureDate(t *testing.T) {
	dir := t.TempDir()
	entries := []Entry{
		{Index: 1, CaptureDate: "2026-04-10", Content: "first"},
		{Index: 2, CaptureDate: "2026-04-10", Content: "second"},
		{Index: 3, CaptureDate: "2026-04-12", Content: "third"},
	}
	if err := Archive(dir, entries); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	got1 := readFile(t, filepath.Join(dir, "raw", "inbox", "2026-04-10.md"))
	if !strings.Contains(got1, "first") || !strings.Contains(got1, "second") {
		t.Errorf("2026-04-10 file missing entries: %q", got1)
	}
	got2 := readFile(t, filepath.Join(dir, "raw", "inbox", "2026-04-12.md"))
	if !strings.Contains(got2, "third") {
		t.Errorf("2026-04-12 file = %q", got2)
	}
	if strings.Contains(got2, "first") {
		t.Error("2026-04-12 file leaked entries from a different date")
	}
}

func TestArchiveAppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	first := []Entry{{Index: 1, CaptureDate: "2026-04-10", Content: "alpha"}}
	if err := Archive(dir, first); err != nil {
		t.Fatal(err)
	}
	second := []Entry{{Index: 1, CaptureDate: "2026-04-10", Content: "beta"}}
	if err := Archive(dir, second); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(dir, "raw", "inbox", "2026-04-10.md"))
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Errorf("appended file missing one of the entries: %q", got)
	}
}

func TestClearInboxStripsEntriesKeepsHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.md")
	body := `# Inbox

Drop captures below.

---

First entry.
---
Second entry.
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := ClearInbox(path)
	if err != nil {
		t.Fatalf("ClearInbox: %v", err)
	}
	if removed == 0 {
		t.Error("removed = 0, want non-zero")
	}
	got := readFile(t, path)
	if strings.Contains(got, "First entry") || strings.Contains(got, "Second entry") {
		t.Errorf("cleared inbox still contains entries:\n%s", got)
	}
	if !strings.Contains(got, "Drop captures") || !strings.Contains(got, "---") {
		t.Errorf("cleared inbox lost header or separator:\n%s", got)
	}
}

func TestClearInboxNoMarkerIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inbox.md")
	body := "just a header, no separator\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := ClearInbox(path)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0 (no marker → no-op)", removed)
	}
	got := readFile(t, path)
	if got != body {
		t.Errorf("file changed: %q", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
