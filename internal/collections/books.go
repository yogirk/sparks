package collections

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// bookHintRE matches `book: Title / Author` lines from inbox archives.
var bookHintRE = regexp.MustCompile(`(?i)^\s*book:\s*(.+?)\s*$`)

// extractBooks derives a book list from two sources:
//   1. Quote attributions (extractQuotes already parses these), where
//      the attribution mentions a recognizable book/source.
//   2. `book:` lines from raw/inbox/*.md archived inbox entries.
//
// The output groups entries by book title. We don't try to canonicalize
// titles — humans use loose conventions. Future enhancement: dedupe via
// a manual aliases file.
func extractBooks(v *vault.Vault, _ *manifest.DB, _ string) (string, error) {
	titles := map[string][]string{} // title -> contexts (quotes or notes)

	// Source 1: quote attributions from raw/quotes/.
	quotes, err := parseQuotesFromGlob(v.Root, DefaultGlobs["Quotes"])
	if err == nil {
		for _, q := range quotes {
			if title := titleFromAttribution(q.Attribution); title != "" {
				snippet := q.Text
				if len(snippet) > 80 {
					snippet = snippet[:77] + "..."
				}
				titles[title] = append(titles[title], "quote: \""+snippet+"\"")
			}
		}
	}

	// Source 2: book: hints from raw/inbox/*.md (archived).
	archives, _ := filepath.Glob(filepath.Join(v.Root, "raw", "inbox", "*.md"))
	sort.Strings(archives)
	for _, path := range archives {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			if m := bookHintRE.FindStringSubmatch(scanner.Text()); m != nil {
				title := strings.TrimSpace(m[1])
				titles[title] = append(titles[title], "noted in "+rel(v.Root, path))
			}
		}
		_ = f.Close()
	}

	var out strings.Builder
	out.WriteString(header("Books"))
	if len(titles) == 0 {
		out.WriteString("_No books referenced yet._\n")
		out.WriteString(trailer())
		return out.String(), nil
	}

	sortedTitles := make([]string, 0, len(titles))
	for t := range titles {
		sortedTitles = append(sortedTitles, t)
	}
	sort.Strings(sortedTitles)

	fmt.Fprintf(&out, "%d distinct book(s) referenced.\n\n", len(sortedTitles))
	for _, t := range sortedTitles {
		fmt.Fprintf(&out, "## %s\n\n", t)
		for _, ctx := range titles[t] {
			fmt.Fprintf(&out, "- %s\n", ctx)
		}
		out.WriteString("\n")
	}
	out.WriteString(trailer())
	return out.String(), nil
}

// titleFromAttribution extracts a probable book title from a quote
// attribution. Heuristic: if the attribution contains ` / ` or `,`,
// the segment after the FIRST one is treated as the source/book.
//
// "Tracy Kidder, The Soul of a New Machine" → "The Soul of a New Machine"
// "Karpathy / LLM Wiki post"                → "LLM Wiki post"
// "anonymous"                               → ""
func titleFromAttribution(attr string) string {
	if attr == "" {
		return ""
	}
	for _, sep := range []string{" / ", ", ", " — "} {
		if i := strings.Index(attr, sep); i > 0 {
			return strings.TrimSpace(attr[i+len(sep):])
		}
	}
	return ""
}
