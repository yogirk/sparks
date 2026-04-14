// Package inbox parses inbox.md into discrete entries and classifies
// deterministic hints (URLs, task markers, book/to-read prefixes, quote
// markers). Semantic labeling is out of scope — this package only does
// pattern matching.
//
// Entry convention:
//
//	# Inbox (optional header)
//	... header text, instructions, comments ...
//	---         <- boundary between header and entries
//	Entry 1 content
//	(optional YYYY-MM-DD capture date on first line)
//	---
//	Entry 2 content
//	---
//	...
package inbox

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

// ErrEmpty signals that the inbox has no real entries — header-only files
// or files with only whitespace between separators. The caller decides
// whether that's an error or a no-op.
var ErrEmpty = errors.New("inbox: no entries to process")

// Entry is one parsed inbox entry. Index is 1-based and stable within a
// single Split call so agents can refer back to entries by number.
type Entry struct {
	Index       int
	CaptureDate string // YYYY-MM-DD — either parsed from first line or filled by caller
	Content     string // trimmed entry body (excluding capture-date line if present)
	Hash        string // sha256 of Content; same content → same hash
	Hints       Hints
}

// Hints are deterministic classifications over an entry's lines. Filling
// these in the CLI saves the agent a regex pass but is explicitly NOT
// semantic labeling — the agent still decides what each hint means.
type Hints struct {
	Tasks     []string `json:"tasks,omitempty"`     // lines starting with `- [ ]` or `TODO:`
	Bookmarks []string `json:"bookmarks,omitempty"` // plain URLs on a line
	HasQuote  bool     `json:"has_quote"`           // lines beginning with `>` or opening-quote + attribution
	HasBook   bool     `json:"has_book"`            // lines starting with `book:` prefix
	HasToRead bool     `json:"has_to_read"`         // lines starting with `to-read:` prefix
}

// SplitOptions configures Split. Today is the fallback capture date when
// an entry has no leading YYYY-MM-DD line. Callers typically pass
// time.Now().Format("2006-01-02").
type SplitOptions struct {
	Today string
}

// Split parses inbox content into entries. Entries with no non-blank
// content are filtered out. The returned slice is empty (nil) if the
// inbox has only a header or is entirely blank.
func Split(content string, opts SplitOptions) []Entry {
	chunks := splitChunks(content)
	if len(chunks) == 0 {
		return nil
	}
	today := opts.Today
	if today == "" {
		today = time.Now().UTC().Format("2006-01-02")
	}
	entries := make([]Entry, 0, len(chunks))
	for _, chunk := range chunks {
		body, date := extractCaptureDate(chunk)
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		if date == "" {
			date = today
		}
		e := Entry{
			Index:       len(entries) + 1,
			CaptureDate: date,
			Content:     body,
			Hash:        hashContent(body),
			Hints:       classify(body),
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

// SplitReader is a streaming wrapper around Split. Reads the entire reader
// into memory; inbox files are small by design (<1 MB even for heavy
// capture sessions) so buffering is fine.
func SplitReader(r io.Reader, opts SplitOptions) ([]Entry, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Split(string(data), opts), nil
}

// splitChunks returns the list of entry bodies from inbox text. The first
// chunk (everything before the first `---` on its own line) is the
// HEADER and is always discarded. Subsequent chunks are entries.
func splitChunks(content string) []string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var chunks []string
	var current strings.Builder
	inHeader := true

	flush := func() {
		if !inHeader {
			chunks = append(chunks, current.String())
		}
		current.Reset()
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "---" {
			flush()
			inHeader = false
			continue
		}
		if !inHeader {
			current.WriteString(line)
			current.WriteByte('\n')
		}
	}
	// Trailing entry (no closing `---`).
	flush()
	return chunks
}

// dateLineRE matches a line that is only a date in YYYY-MM-DD form,
// optionally surrounded by whitespace.
var dateLineRE = regexp.MustCompile(`^\s*(\d{4}-\d{2}-\d{2})\s*$`)

// extractCaptureDate peels a leading YYYY-MM-DD line off the entry, if
// present. Leading blank lines are allowed (and preserved in the returned
// body if the date isn't found). Returns the cleaned body and the
// extracted date (empty if none found).
func extractCaptureDate(chunk string) (body, date string) {
	lines := strings.Split(chunk, "\n")
	// Find the first non-blank line.
	first := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			first = i
			break
		}
	}
	if first < 0 {
		return chunk, ""
	}
	m := dateLineRE.FindStringSubmatch(lines[first])
	if m == nil {
		return chunk, ""
	}
	// Drop lines[0..first] (leading blanks + the date line itself).
	return strings.Join(lines[first+1:], "\n"), m[1]
}

func hashContent(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
