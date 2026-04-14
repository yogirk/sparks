package inbox

import (
	"regexp"
	"strings"
)

// Hint patterns. These are deterministic and pedestrian on purpose — the
// rule is "any agent could compute this with grep," not "a smart parser."
// Anything squishier belongs in the agent, not here.

var (
	// urlRE matches http(s) URLs. Captures the full URL including the
	// scheme. Trailing punctuation (. , ! ? ) ] ") is stripped because
	// prose often wraps URLs in punctuation.
	urlRE = regexp.MustCompile(`https?://[^\s<>"'\x60]+`)

	// taskCheckboxRE matches `- [ ]` or `* [ ]` markdown checkboxes, with
	// or without leading whitespace.
	taskCheckboxRE = regexp.MustCompile(`^\s*[-*]\s+\[\s\]\s+(.+)$`)

	// taskTodoRE matches lines starting with `TODO:` (case-insensitive).
	taskTodoRE = regexp.MustCompile(`(?i)^\s*TODO:\s*(.+)$`)

	// bookPrefixRE matches a line starting with `book:`.
	bookPrefixRE = regexp.MustCompile(`(?i)^\s*book:\s*(.+)$`)

	// toReadPrefixRE matches a line starting with `to-read:`.
	toReadPrefixRE = regexp.MustCompile(`(?i)^\s*to-read:\s*(.+)$`)

	// quoteBlockRE matches a markdown blockquote line.
	quoteBlockRE = regexp.MustCompile(`^\s*>\s+\S`)

	// quotedAttributionRE matches a line ending with `— Author` or `-- Author`
	// following a quoted sentence. Rough; good enough for hint purposes.
	quotedAttributionRE = regexp.MustCompile(`["'\x60].*["'\x60]\s*[—–-]{1,2}\s+\S`)
)

// trailingPunct lists runes we strip from URL matches. Split out for
// readability; Go's strings.TrimRight wants a cutset string.
const trailingPunct = ".,;:!?)]}'\""

// classify walks an entry body line-by-line, filling Hints based on
// purely syntactic matches. No model calls, no network, no surprises.
func classify(body string) Hints {
	var h Hints
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimRight(raw, "\r")

		// Tasks: checkbox form.
		if m := taskCheckboxRE.FindStringSubmatch(line); m != nil {
			h.Tasks = append(h.Tasks, strings.TrimSpace(m[1]))
			continue
		}
		// Tasks: TODO: prefix form.
		if m := taskTodoRE.FindStringSubmatch(line); m != nil {
			h.Tasks = append(h.Tasks, strings.TrimSpace(m[1]))
			continue
		}

		// Book / to-read prefixes. These are mutually exclusive per line
		// so we can check them before URLs and quotes.
		if bookPrefixRE.MatchString(line) {
			h.HasBook = true
		}
		if toReadPrefixRE.MatchString(line) {
			h.HasToRead = true
		}

		// Quote markers.
		if quoteBlockRE.MatchString(line) || quotedAttributionRE.MatchString(line) {
			h.HasQuote = true
		}

		// URLs. A single line may have multiple; dedupe at the end.
		for _, m := range urlRE.FindAllString(line, -1) {
			clean := strings.TrimRight(m, trailingPunct)
			if clean != "" {
				h.Bookmarks = append(h.Bookmarks, clean)
			}
		}
	}

	h.Bookmarks = dedupStrings(h.Bookmarks)
	return h
}

func dedupStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
