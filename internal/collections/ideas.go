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

// extractIdeas walks the configured glob (default raw/ideas/) and emits
// one bullet per file. Title is the file's first H1 or its filename.
// Status hint comes from a `status: X` line if present (free-form).
func extractIdeas(v *vault.Vault, _ *manifest.DB, glob string) (string, error) {
	files, err := expandGlob(v.Root, glob)
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	type entry struct {
		title, status, source string
	}
	var entries []entry

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		title := titleFromFilename(path)
		status := ""
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
			if strings.HasPrefix(strings.ToLower(line), "status:") {
				status = strings.TrimSpace(line[len("status:"):])
			}
		}
		_ = f.Close()
		entries = append(entries, entry{title: title, status: status, source: rel(v.Root, path)})
	}

	var out strings.Builder
	out.WriteString(header("Ideas"))
	if len(entries) == 0 {
		out.WriteString("_No ideas captured yet._\n")
		out.WriteString(trailer())
		return out.String(), nil
	}

	// Group by status so the page reads as a portfolio view.
	byStatus := map[string][]entry{}
	for _, e := range entries {
		key := e.status
		if key == "" {
			key = "uncategorized"
		}
		byStatus[strings.ToLower(key)] = append(byStatus[strings.ToLower(key)], e)
	}
	statusOrder := []string{"active", "shipping", "shipped", "backlog", "parked", "done", "uncategorized"}
	seen := map[string]bool{}

	fmt.Fprintf(&out, "%d idea(s) tracked.\n\n", len(entries))
	for _, s := range statusOrder {
		if list, ok := byStatus[s]; ok {
			seen[s] = true
			fmt.Fprintf(&out, "## %s\n\n", titleCase(s))
			for _, e := range list {
				fmt.Fprintf(&out, "- %s _(`%s`)_\n", e.title, e.source)
			}
			out.WriteString("\n")
		}
	}
	// Anything outside the canonical buckets gets surfaced too.
	var extras []string
	for s := range byStatus {
		if !seen[s] {
			extras = append(extras, s)
		}
	}
	sort.Strings(extras)
	for _, s := range extras {
		fmt.Fprintf(&out, "## %s\n\n", titleCase(s))
		for _, e := range byStatus[s] {
			fmt.Fprintf(&out, "- %s _(`%s`)_\n", e.title, e.source)
		}
		out.WriteString("\n")
	}

	out.WriteString(trailer())
	return out.String(), nil
}

// titleCase upper-cases the first byte of an ASCII string. Sufficient
// for our status labels — all latin lowercase by convention.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}
