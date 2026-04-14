package collections

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// extractProjects queries the manifest for wiki entities tagged with
// `project` and groups them by maturity (which we interpret as ship
// status: shipping = stable, building = working, exploring = seed,
// archived = historical). The mapping is deliberate: maturity already
// encodes "how settled is this," which is exactly what a portfolio
// view wants.
func extractProjects(v *vault.Vault, db *manifest.DB, _ string) (string, error) {
	pages, err := db.ListPages()
	if err != nil {
		return "", err
	}

	type proj struct {
		title    string
		path     string
		maturity string
	}
	var projects []proj
	for _, p := range pages {
		if p.Type != "entity" {
			continue
		}
		if !hasProjectTag(p.Path, db) {
			continue
		}
		projects = append(projects, proj{
			title:    coalesce(p.Title, titleFromFilename(p.Path)),
			path:     p.Path,
			maturity: p.Maturity,
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].title) < strings.ToLower(projects[j].title)
	})

	var out strings.Builder
	out.WriteString(header("Projects"))
	if len(projects) == 0 {
		out.WriteString("_No project entities yet. Tag a wiki entity with `project` to surface it here._\n")
		out.WriteString(trailer())
		return out.String(), nil
	}

	byMaturity := map[string][]proj{}
	for _, p := range projects {
		byMaturity[p.maturity] = append(byMaturity[p.maturity], p)
	}
	maturityOrder := []string{"working", "stable", "seed", "historical"}
	maturityLabel := map[string]string{
		"working":    "Building",
		"stable":     "Shipped",
		"seed":       "Exploring",
		"historical": "Archived",
	}

	fmt.Fprintf(&out, "%d project(s) tracked.\n\n", len(projects))
	for _, m := range maturityOrder {
		list := byMaturity[m]
		if len(list) == 0 {
			continue
		}
		label := maturityLabel[m]
		if label == "" {
			label = m
		}
		fmt.Fprintf(&out, "## %s (%d)\n\n", label, len(list))
		for _, p := range list {
			fmt.Fprintf(&out, "- [[%s]]\n", strings.TrimSuffix(strings.TrimPrefix(p.path, "wiki/"), ".md"))
		}
		out.WriteString("\n")
	}
	out.WriteString(trailer())
	return out.String(), nil
}

// hasProjectTag returns true if the given page's tags JSON includes
// "project". We re-query the row instead of plumbing tags through
// PageInfo to keep that struct slim — this is the only consumer.
func hasProjectTag(path string, db *manifest.DB) bool {
	pages, err := db.ListPages()
	if err != nil {
		return false
	}
	// Tags aren't on PageInfo; fetch via a direct lookup. To avoid
	// adding another method, we reuse the JSON tags field stored in the
	// frontmatter table. Falls back to false on any decoding error.
	for _, p := range pages {
		if p.Path != path {
			continue
		}
		// We need the tags. They're not in PageInfo. Read them via
		// the manifest's underlying JSON (cheaper than a new method
		// for a single check; revisit if Projects collection grows).
		_ = p
	}
	tags, _ := loadTags(db, path)
	for _, t := range tags {
		if strings.EqualFold(t, "project") {
			return true
		}
	}
	return false
}

// loadTags hits the manifest directly for the JSON tags array of one
// path. Single-use convenience to avoid bloating PageInfo.
func loadTags(db *manifest.DB, path string) ([]string, error) {
	row := db.Raw().QueryRow(`SELECT tags FROM frontmatter WHERE path = ?`, path)
	var tagsJSON string
	if err := row.Scan(&tagsJSON); err != nil {
		return nil, err
	}
	var out []string
	if err := json.Unmarshal([]byte(tagsJSON), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
