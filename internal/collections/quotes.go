package collections

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// quoteAttributionRE matches a line like "— Author, Source" or
// "-- Author / Book Title". The em dash variant is preferred but we
// accept double-hyphen too. Captured group is the attribution body.
var quoteAttributionRE = regexp.MustCompile(`^\s*[—–-]{1,2}\s+(.+)$`)

// QuoteRecord is one quote with its source attribution. Exported so the
// Books extractor can reuse the parsed result.
type QuoteRecord struct {
	Text        string
	Attribution string
	SourcePath  string // vault-relative path of the raw file
}

// extractQuotes walks the configured glob (default raw/quotes/) and
// parses blockquote-style entries:
//
//	> First line of the quote
//	> spanning multiple lines.
//	— Author, Source
//
// Quotes without attributions are still emitted; missing attribution
// becomes the literal string "(no attribution)".
func extractQuotes(v *vault.Vault, _ *manifest.DB, glob string) (string, error) {
	records, err := parseQuotesFromGlob(v.Root, glob)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	out.WriteString(header("Quotes"))
	if len(records) == 0 {
		out.WriteString("_No quotes captured yet._\n")
		out.WriteString(trailer())
		return out.String(), nil
	}

	fmt.Fprintf(&out, "%d quotes captured.\n\n", len(records))
	for _, r := range records {
		out.WriteString("> ")
		out.WriteString(strings.ReplaceAll(r.Text, "\n", "\n> "))
		out.WriteString("\n")
		if r.Attribution != "" {
			fmt.Fprintf(&out, "— %s\n\n", r.Attribution)
		} else {
			out.WriteString("— (no attribution)\n\n")
		}
	}
	out.WriteString(trailer())
	return out.String(), nil
}

// parseQuotesFromGlob is exposed at package scope so the Books extractor
// can reuse the parsed quote stream without re-walking the filesystem.
func parseQuotesFromGlob(vaultRoot, glob string) ([]QuoteRecord, error) {
	files, err := expandGlob(vaultRoot, glob)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var out []QuoteRecord
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		out = append(out, parseQuotesFile(f, rel(vaultRoot, path))...)
		_ = f.Close()
	}
	return out, nil
}

// parseQuotesFile streams lines and assembles QuoteRecords. State machine:
//
//	NORMAL → see `> ` → enter QUOTE, accumulate text
//	QUOTE  → see `> ` continuation → accumulate
//	QUOTE  → see attribution → close record
//	QUOTE  → see blank line   → close record (no attribution)
func parseQuotesFile(r interface{ Read([]byte) (int, error) }, sourcePath string) []QuoteRecord {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var out []QuoteRecord
	var quote strings.Builder
	inQuote := false

	flush := func(attribution string) {
		if !inQuote {
			return
		}
		text := strings.TrimSpace(quote.String())
		if text != "" {
			out = append(out, QuoteRecord{
				Text:        text,
				Attribution: strings.TrimSpace(attribution),
				SourcePath:  sourcePath,
			})
		}
		quote.Reset()
		inQuote = false
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "> ") {
			body := strings.TrimLeft(strings.TrimLeft(line, " \t")[2:], " ")
			if inQuote {
				quote.WriteString("\n")
			}
			quote.WriteString(body)
			inQuote = true
			continue
		}
		if m := quoteAttributionRE.FindStringSubmatch(line); m != nil && inQuote {
			flush(m[1])
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush("")
		}
	}
	flush("") // EOF
	return out
}
