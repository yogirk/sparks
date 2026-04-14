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

// toReadHintRE matches `to-read: <something>` lines in inbox archives.
var toReadHintRE = regexp.MustCompile(`(?i)^\s*to-read:\s*(.+?)\s*$`)

// extractReadingList derives a queue of to-read items from `to-read:`
// lines in archived inbox entries (raw/inbox/YYYY-MM-DD.md). Each entry
// is unique — subsequent duplicates are dropped to keep the list useful
// rather than chronological.
func extractReadingList(v *vault.Vault, _ *manifest.DB, _ string) (string, error) {
	type entry struct {
		text   string
		source string
	}
	var entries []entry
	seen := map[string]bool{}

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
			if m := toReadHintRE.FindStringSubmatch(scanner.Text()); m != nil {
				text := strings.TrimSpace(m[1])
				if text == "" || seen[text] {
					continue
				}
				seen[text] = true
				entries = append(entries, entry{text: text, source: rel(v.Root, path)})
			}
		}
		_ = f.Close()
	}

	var out strings.Builder
	out.WriteString(header("Reading List"))
	if len(entries) == 0 {
		out.WriteString("_No to-read items captured yet._\n")
		out.WriteString(trailer())
		return out.String(), nil
	}

	fmt.Fprintf(&out, "%d item(s) in queue.\n\n", len(entries))
	for _, e := range entries {
		fmt.Fprintf(&out, "- %s _(via %s)_\n", e.text, e.source)
	}
	out.WriteString(trailer())
	return out.String(), nil
}
