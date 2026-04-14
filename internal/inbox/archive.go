package inbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Archive appends each entry to `raw/inbox/YYYY-MM-DD.md` inside vaultRoot,
// grouped by CaptureDate. Existing date files are appended to (not replaced).
// This lets ingests run multiple times per day without losing history.
//
// Each archived entry is delimited by a `---` separator, mirroring inbox.md
// format so archive files remain round-trippable.
func Archive(vaultRoot string, entries []Entry) error {
	archiveDir := filepath.Join(vaultRoot, "raw", "inbox")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("mkdir archive: %w", err)
	}

	byDate := make(map[string][]Entry, 8)
	for _, e := range entries {
		byDate[e.CaptureDate] = append(byDate[e.CaptureDate], e)
	}

	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	for _, date := range dates {
		path := filepath.Join(archiveDir, date+".md")
		if err := appendEntries(path, byDate[date]); err != nil {
			return err
		}
	}
	return nil
}

// ClearInbox rewrites inbox.md to contain only the header (everything up
// to and including the first `---` line). All entries after that line are
// removed, since they've been archived. The trailing blank line is
// preserved so the user's next capture doesn't butt up against the `---`.
//
// Returns the number of bytes removed. If no `---` exists in the file,
// returns nil without modifying anything — nothing was archive-able to
// begin with.
func ClearInbox(inboxPath string) (int, error) {
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		return 0, err
	}
	marker := "\n---\n"
	idx := strings.Index(string(data), marker)
	if idx < 0 {
		// Try file-start edge case (first line is `---`).
		if strings.HasPrefix(string(data), "---\n") {
			idx = 0
			marker = "---\n"
		} else {
			return 0, nil
		}
	}
	header := string(data[:idx+len(marker)])
	// Always end header with a blank line so next capture is separated.
	if !strings.HasSuffix(header, "\n\n") {
		header += "\n"
	}
	removed := len(data) - len(header)
	if removed <= 0 {
		return 0, nil
	}
	if err := os.WriteFile(inboxPath, []byte(header), 0o644); err != nil {
		return 0, err
	}
	return removed, nil
}

func appendEntries(path string, entries []Entry) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	for _, e := range entries {
		if _, err := fmt.Fprintf(f, "---\n%s\n", strings.TrimRight(e.Content, "\n")); err != nil {
			return fmt.Errorf("write entry %d: %w", e.Index, err)
		}
	}
	return nil
}
