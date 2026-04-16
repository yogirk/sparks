package view

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yogirk/sparks/internal/manifest"
)

// sidecar holds the panels that flank every rendered page. Keeping
// this plumbing in its own file so server.go stays focused on routing.

// navSection is one type-group in the left sidebar.
type navSection struct {
	Type  string
	Label string
	Count int
	Pages []navEntry
}

type navEntry struct {
	Title string
	Href  string
	Path  string
}

// backlink is one inbound reference to the current page.
type backlink struct {
	Title string
	Href  string
	Path  string
}

// recentPage is an entry in the "Recent" panel on the index.
type recentPage struct {
	Title   string
	Href    string
	Type    string
	Updated string
}

// tagChip is a clickable tag used in the right sidebar + tag page.
type tagChip struct {
	Name  string
	Count int
	Href  string
}

// pageMeta is the right-sidebar metadata block for a single page.
type pageMeta struct {
	Type     string
	Maturity string
	Created  string
	Updated  string
	Tags     []tagChip
	Aliases  []string
	Sources  []string
}

// buildNav returns the left-sidebar page-tree built from ListPages.
// Pages without a type (collections, raw files) are excluded — those
// are reached via the reading column or the "Collections" type label.
func buildNav(pages []manifest.PageInfo) []navSection {
	byType := map[string][]navEntry{}
	for _, p := range pages {
		if p.Type == "" {
			continue
		}
		title := p.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(p.Path), ".md")
		}
		byType[p.Type] = append(byType[p.Type], navEntry{
			Title: title,
			Href:  pathToHref(p.Path),
			Path:  p.Path,
		})
	}
	for t := range byType {
		sort.Slice(byType[t], func(i, j int) bool {
			return strings.ToLower(byType[t][i].Title) < strings.ToLower(byType[t][j].Title)
		})
	}
	sections := make([]navSection, 0, len(typeOrder))
	for _, t := range typeOrder {
		list, ok := byType[t]
		if !ok {
			continue
		}
		sections = append(sections, navSection{
			Type:  t,
			Label: typeLabels[t],
			Count: len(list),
			Pages: list,
		})
	}
	return sections
}

// buildBacklinks returns pages that link to `toPath`. Uses the manifest
// wikilinks table; index.md is excluded since it's a catalog.
func buildBacklinks(db *manifest.DB, pages []manifest.PageInfo, toPath string) []backlink {
	edges, err := db.IncomingLinks(toPath)
	if err != nil {
		return nil
	}
	// Build a path→(title) index so backlinks can display the source
	// page's title rather than its raw file path.
	titleByPath := make(map[string]string, len(pages))
	for _, p := range pages {
		t := p.Title
		if t == "" {
			t = strings.TrimSuffix(filepath.Base(p.Path), ".md")
		}
		titleByPath[p.Path] = t
	}
	seen := map[string]bool{}
	var out []backlink
	for _, e := range edges {
		if e.Source == "" || seen[e.Source] {
			continue
		}
		if filepath.Base(e.Source) == "index.md" {
			continue
		}
		title, ok := titleByPath[e.Source]
		if !ok {
			title = strings.TrimSuffix(filepath.Base(e.Source), ".md")
		}
		out = append(out, backlink{
			Title: title,
			Href:  pathToHref(e.Source),
			Path:  e.Source,
		})
		seen[e.Source] = true
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out
}

// buildRecent returns the N most-recently-updated pages. "Recent"
// means `updated:` frontmatter date, descending; ties broken by path.
// Pages with no updated date sort last.
func buildRecent(pages []manifest.PageInfo, limit int) []recentPage {
	list := make([]recentPage, 0, len(pages))
	for _, p := range pages {
		if p.Type == "" {
			continue
		}
		title := p.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(p.Path), ".md")
		}
		list = append(list, recentPage{
			Title:   title,
			Href:    pathToHref(p.Path),
			Type:    p.Type,
			Updated: p.Updated,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i].Updated, list[j].Updated
		if a == b {
			return list[i].Title < list[j].Title
		}
		// Newer dates come first. Empty dates sort last.
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a > b
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

// buildTagIndex counts tag usage across pages and returns a sorted list
// of chips (by count desc, then alpha). Callers decide how many to show.
func buildTagIndex(db *manifest.DB, pages []manifest.PageInfo) []tagChip {
	counts := map[string]int{}
	for _, p := range pages {
		tags, _ := loadTags(db, p.Path)
		for _, t := range tags {
			t = strings.ToLower(strings.TrimSpace(t))
			if t == "" {
				continue
			}
			counts[t]++
		}
	}
	out := make([]tagChip, 0, len(counts))
	for name, n := range counts {
		out = append(out, tagChip{Name: name, Count: n, Href: "/tags/" + name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// buildPageMeta assembles the right-sidebar metadata block.
func buildPageMeta(db *manifest.DB, path string, fmType, maturity, created, updated string, aliases, sources []string) pageMeta {
	tags, _ := loadTags(db, path)
	chips := make([]tagChip, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		chips = append(chips, tagChip{Name: t, Href: "/tags/" + t})
	}
	return pageMeta{
		Type:     fmType,
		Maturity: maturity,
		Created:  created,
		Updated:  updated,
		Tags:     chips,
		Aliases:  aliases,
		Sources:  sources,
	}
}

// loadTags re-queries the manifest for a page's tags (stored as JSON).
// Kept here rather than on *DB because Projects collection already
// has a duplicate in internal/collections; a shared helper would be
// nice later.
func loadTags(db *manifest.DB, path string) ([]string, error) {
	row := db.Raw().QueryRow(`SELECT tags FROM frontmatter WHERE path = ?`, path)
	var raw string
	if err := row.Scan(&raw); err != nil {
		return nil, err
	}
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out, nil
}

// --- tag page ---

// tagPageData is what the /tags/{tag} template receives.
type tagPageData struct {
	VaultName string
	Tag       string
	Nav       []navSection
	Recent    []recentPage
	Tags      []tagChip
	Pages     []navEntry
}

// buildTagPages returns the pages whose tags include tag (case-insensitive).
func buildTagPages(db *manifest.DB, pages []manifest.PageInfo, tag string) []navEntry {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return nil
	}
	var out []navEntry
	for _, p := range pages {
		tags, _ := loadTags(db, p.Path)
		match := false
		for _, t := range tags {
			if strings.EqualFold(strings.TrimSpace(t), tag) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		title := p.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(p.Path), ".md")
		}
		out = append(out, navEntry{
			Title: title,
			Href:  pathToHref(p.Path),
			Path:  p.Path,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out
}

// sinceHuman renders an absolute date string "2026-04-10" as a friendly
// relative label ("3 days ago", "today"). Falls back to the raw string
// if parsing fails — we never want the UI to go blank.
func sinceHuman(updated string) string {
	if updated == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", updated)
	if err != nil {
		return updated
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return (updated)
	default:
		return updated
	}
}
