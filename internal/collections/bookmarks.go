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

// urlRE matches http(s) URLs. Mirror of the inbox classifier; duplicated
// here so collections can grow its own URL semantics without coupling.
var urlRE = regexp.MustCompile(`https?://[^\s<>"'\x60]+`)

const trailingPunct = ".,;:!?)]}'\""

// extractBookmarks walks the configured glob (default raw/weblinks/),
// pulls every URL with a best-effort description, and writes a flat
// list grouped by source file. Description is the line containing the
// URL minus the URL itself, trimmed.
func extractBookmarks(v *vault.Vault, _ *manifest.DB, glob string) (string, error) {
	files, err := expandGlob(v.Root, glob)
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	type entry struct {
		url, description string
	}
	bySource := map[string][]entry{}
	srcOrder := []string{}

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		var entries []entry
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			matches := urlRE.FindAllString(line, -1)
			if len(matches) == 0 {
				continue
			}
			for _, u := range matches {
				clean := strings.TrimRight(u, trailingPunct)
				desc := strings.TrimSpace(strings.Replace(line, u, "", 1))
				desc = strings.TrimLeft(desc, "-*> ")
				entries = append(entries, entry{url: clean, description: desc})
			}
		}
		_ = f.Close()
		if len(entries) > 0 {
			rp := rel(v.Root, path)
			bySource[rp] = entries
			srcOrder = append(srcOrder, rp)
		}
	}

	var out strings.Builder
	out.WriteString(header("Bookmarks"))
	if len(srcOrder) == 0 {
		out.WriteString("_No bookmarks captured yet._\n")
		out.WriteString(trailer())
		return out.String(), nil
	}

	totalURLs := 0
	for _, src := range srcOrder {
		totalURLs += len(bySource[src])
	}
	fmt.Fprintf(&out, "%d bookmarks across %d source file(s).\n\n", totalURLs, len(srcOrder))

	for _, src := range srcOrder {
		fmt.Fprintf(&out, "## %s\n\n", titleFromFilename(src))
		for _, e := range bySource[src] {
			if e.description != "" {
				fmt.Fprintf(&out, "- [%s](%s) — %s\n", e.url, e.url, e.description)
			} else {
				fmt.Fprintf(&out, "- [%s](%s)\n", e.url, e.url)
			}
		}
		out.WriteString("\n")
	}
	out.WriteString(trailer())
	return out.String(), nil
}
