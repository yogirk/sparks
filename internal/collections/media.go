package collections

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// extractMedia walks the configured glob (default raw/media/*.md),
// emits one entry per file with the file's H1 (or filename) as the
// title and the first URL on the page as the link.
func extractMedia(v *vault.Vault, _ *manifest.DB, glob string) (string, error) {
	files, err := expandGlob(v.Root, glob)
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	type entry struct {
		title, url, source string
	}
	var entries []entry

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		title := titleFromFilename(path)
		var firstURL string
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "# ") && title == titleFromFilename(path) {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
			if firstURL == "" {
				if m := urlRE.FindString(line); m != "" {
					firstURL = strings.TrimRight(m, trailingPunct)
				}
			}
		}
		_ = f.Close()
		entries = append(entries, entry{title: title, url: firstURL, source: rel(v.Root, path)})
	}

	var out strings.Builder
	out.WriteString(header("Media"))
	if len(entries) == 0 {
		out.WriteString("_No media notes captured yet._\n")
		out.WriteString(trailer())
		return out.String(), nil
	}

	fmt.Fprintf(&out, "%d media note(s).\n\n", len(entries))
	for _, e := range entries {
		if e.url != "" {
			fmt.Fprintf(&out, "- [[%s|%s]] — [link](%s)\n", strings.TrimSuffix(strings.TrimPrefix(e.source, "raw/"), ".md"), e.title, e.url)
		} else {
			fmt.Fprintf(&out, "- [[%s|%s]]\n", strings.TrimSuffix(strings.TrimPrefix(e.source, "raw/"), ".md"), e.title)
		}
	}
	out.WriteString(trailer())
	return out.String(), nil
}
