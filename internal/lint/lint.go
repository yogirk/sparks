// Package lint produces a deterministic vault health report. Every check
// is pure state over the manifest and filesystem — no language model,
// no heuristics that drift run-to-run.
//
// Checks implemented (all in one Run pass):
//
//  1. orphans            : wiki pages no other page links to
//  2. broken-links       : wikilinks whose target doesn't resolve
//  3. missing-frontmatter: wiki pages missing a required field
//  4. invalid-frontmatter: wiki pages with bad enum or date format
//  5. thin-pages         : fewer than 3 sentences in body
//  6. stale-pages        : wiki updated older than max source mtime
//  7. dead-sources       : sources: paths not present in vault
//  8. duplicate-aliases  : two or more pages claiming the same alias
package lint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yogirk/sparks/internal/frontmatter"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// Issue is a single lint finding. Check is one of the canonical names
// listed above. Path is vault-relative.
type Issue struct {
	Check   string `json:"check"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// Report is the full lint output. Counts groups issues by check for easy
// summary rendering.
type Report struct {
	Issues []Issue        `json:"issues"`
	Counts map[string]int `json:"counts"`
}

// checkNames are the canonical check identifiers in display order.
var checkNames = []string{
	"orphans",
	"broken-links",
	"missing-frontmatter",
	"invalid-frontmatter",
	"thin-pages",
	"stale-pages",
	"dead-sources",
	"duplicate-aliases",
}

// Run executes every check against the given vault + manifest. The
// manifest is expected to be fresh (call `sparks scan` first).
func Run(v *vault.Vault, db *manifest.DB) (Report, error) {
	pages, err := db.ListPages()
	if err != nil {
		return Report{}, err
	}

	var issues []Issue

	issues = append(issues, checkOrphansAndBrokenLinks(db, pages)...)
	issues = append(issues, checkFrontmatter(pages)...)
	issues = append(issues, checkPagesWithoutFrontmatter(db)...)
	issues = append(issues, checkThinAndStale(v, pages)...)
	issues = append(issues, checkDeadSources(v, pages)...)
	issues = append(issues, checkDuplicateAliases(pages)...)

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Check != issues[j].Check {
			return issues[i].Check < issues[j].Check
		}
		return issues[i].Path < issues[j].Path
	})

	counts := make(map[string]int, len(checkNames))
	for _, c := range checkNames {
		counts[c] = 0
	}
	for _, iss := range issues {
		counts[iss.Check]++
	}
	return Report{Issues: issues, Counts: counts}, nil
}

// checkOrphansAndBrokenLinks runs the graph-dependent checks together
// since they share the same data.
func checkOrphansAndBrokenLinks(db *manifest.DB, pages []manifest.PageInfo) []Issue {
	var issues []Issue

	broken, err := db.BrokenLinks()
	if err == nil {
		for _, e := range broken {
			issues = append(issues, Issue{
				Check:   "broken-links",
				Path:    e.Source,
				Message: "wikilink target not found: [[" + e.Target + "]]",
			})
		}
	}

	// Orphans: wiki pages that no other page links to. Pages linked only
	// from index.md are still orphans from the graph's perspective —
	// index is a catalog, not semantic context.
	for _, p := range pages {
		if filepath.Base(p.Path) == "index.md" {
			continue
		}
		incoming, err := db.IncomingLinks(p.Path)
		if err != nil {
			continue
		}
		realIncoming := 0
		for _, e := range incoming {
			if filepath.Base(e.Source) == "index.md" {
				continue
			}
			realIncoming++
		}
		if realIncoming == 0 {
			issues = append(issues, Issue{
				Check:   "orphans",
				Path:    p.Path,
				Message: "no incoming wikilinks from other pages",
			})
		}
	}
	return issues
}

// checkPagesWithoutFrontmatter flags wiki .md files that have no
// frontmatter block at all. These slip past checkFrontmatter (which
// joins on the frontmatter table) so we ask the manifest directly.
func checkPagesWithoutFrontmatter(db *manifest.DB) []Issue {
	paths, err := db.WikiFilesWithoutFrontmatter()
	if err != nil {
		return nil
	}
	var issues []Issue
	for _, p := range paths {
		// index.md is allowed to lack frontmatter; it's a generated catalog.
		if filepath.Base(p) == "index.md" || filepath.Base(p) == "log.md" {
			continue
		}
		issues = append(issues, Issue{
			Check:   "missing-frontmatter",
			Path:    p,
			Message: "wiki page has no frontmatter block",
		})
	}
	return issues
}

// checkFrontmatter validates required fields, enums, and date formats.
func checkFrontmatter(pages []manifest.PageInfo) []Issue {
	var issues []Issue
	for _, p := range pages {
		fm := frontmatter.Frontmatter{
			Title:    p.Title,
			Type:     p.Type,
			Maturity: p.Maturity,
			Aliases:  p.Aliases,
			Sources:  p.Sources,
			Created:  p.Created,
			Updated:  p.Updated,
		}
		for _, msg := range frontmatter.Validate(fm) {
			check := "invalid-frontmatter"
			if strings.HasPrefix(msg, "missing required field") {
				check = "missing-frontmatter"
			}
			issues = append(issues, Issue{Check: check, Path: p.Path, Message: msg})
		}
		for _, field := range []struct {
			name string
			v    string
		}{{"created", p.Created}, {"updated", p.Updated}} {
			if field.v == "" {
				continue
			}
			if !isValidDate(field.v) {
				issues = append(issues, Issue{
					Check:   "invalid-frontmatter",
					Path:    p.Path,
					Message: "bad " + field.name + " date format (want YYYY-MM-DD): " + field.v,
				})
			}
		}
	}
	return issues
}

// checkThinAndStale reads the body once and runs both body-dependent checks.
func checkThinAndStale(v *vault.Vault, pages []manifest.PageInfo) []Issue {
	var issues []Issue
	for _, p := range pages {
		full := filepath.Join(v.Root, p.Path)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		_, body, _ := frontmatter.ParseBytes(data)
		if body == nil {
			body = data
		}
		if countSentences(string(body)) < 3 {
			issues = append(issues, Issue{
				Check:   "thin-pages",
				Path:    p.Path,
				Message: "fewer than 3 sentences of content",
			})
		}
		if isStale(v, p) {
			issues = append(issues, Issue{
				Check:   "stale-pages",
				Path:    p.Path,
				Message: "wiki updated older than source file mtime",
			})
		}
	}
	return issues
}

// isStale reports whether any listed source file's mtime is newer than
// the page's `updated` frontmatter date. Missing dates or missing source
// files are not considered stale — those are caught by other checks.
func isStale(v *vault.Vault, p manifest.PageInfo) bool {
	if p.Updated == "" || len(p.Sources) == 0 {
		return false
	}
	updated, err := time.Parse("2006-01-02", p.Updated)
	if err != nil {
		return false
	}
	// Compare against end-of-day on updated so source mtimes captured
	// later the same day don't trip staleness.
	updatedEOD := updated.Add(24 * time.Hour)
	for _, src := range p.Sources {
		info, err := os.Stat(filepath.Join(v.Root, src))
		if err != nil {
			continue
		}
		if info.ModTime().After(updatedEOD) {
			return true
		}
	}
	return false
}

// checkDeadSources flags wiki pages whose sources: references a path
// that doesn't exist on disk.
func checkDeadSources(v *vault.Vault, pages []manifest.PageInfo) []Issue {
	var issues []Issue
	for _, p := range pages {
		for _, src := range p.Sources {
			if src == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(v.Root, src)); err != nil {
				issues = append(issues, Issue{
					Check:   "dead-sources",
					Path:    p.Path,
					Message: "source not found: " + src,
				})
			}
		}
	}
	return issues
}

// checkDuplicateAliases reports any alias claimed by more than one page.
// Aliases are identity keys for dedup — two pages claiming the same one
// means the resolver is non-deterministic, which is a real bug.
func checkDuplicateAliases(pages []manifest.PageInfo) []Issue {
	owners := map[string][]string{} // folded alias -> owning paths
	for _, p := range pages {
		for _, a := range p.Aliases {
			key := strings.ToLower(strings.TrimSpace(a))
			if key == "" {
				continue
			}
			owners[key] = append(owners[key], p.Path)
		}
	}
	var issues []Issue
	for alias, paths := range owners {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		for _, p := range paths {
			issues = append(issues, Issue{
				Check:   "duplicate-aliases",
				Path:    p,
				Message: "alias \"" + alias + "\" also claimed by: " + strings.Join(without(paths, p), ", "),
			})
		}
	}
	return issues
}

// without returns paths minus p (stable order).
func without(paths []string, p string) []string {
	out := make([]string, 0, len(paths))
	for _, x := range paths {
		if x != p {
			out = append(out, x)
		}
	}
	return out
}

// isValidDate checks the YYYY-MM-DD format. Strict: no trailing time,
// no alternate separators.
func isValidDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// countSentences is a pedestrian sentence counter. Markdown prose with
// 3+ sentence-ending punctuation marks (. ! ?) passes. Falls through
// code fences and frontmatter-less content. Not perfect, but pedagogical:
// the point is to flag stub pages for expansion, not grade essays.
func countSentences(body string) int {
	body = strings.ReplaceAll(body, "\r", "")
	lines := strings.Split(body, "\n")
	inFence := false
	count := 0
	for _, l := range lines {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trim, "#") || trim == "" {
			continue
		}
		for _, r := range trim {
			switch r {
			case '.', '!', '?':
				count++
			}
		}
	}
	return count
}
