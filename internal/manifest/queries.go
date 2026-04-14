package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// QueryFilter describes the structured-lookup parameters supported by
// `sparks query`. All fields are optional; non-empty fields combine via
// AND. When everything is empty, ListPages is the equivalent.
type QueryFilter struct {
	Title       string
	Alias       string
	Tag         string
	Type        string
	Maturity    string
	LinkedFrom  string
	LinksTo     string
	Stale       bool
	Orphan      bool
	Thin        bool
}

// Query runs a structured lookup over the manifest and returns matching
// pages. We do the simple checks in SQL and the in-memory ones (alias,
// tag membership, link graph traversal) in Go to keep the SQL readable.
func (d *DB) Query(v Vaultish, f QueryFilter) ([]PageInfo, error) {
	pages, err := d.ListPages()
	if err != nil {
		return nil, err
	}

	out := pages[:0]
	for _, p := range pages {
		if f.Title != "" && !strings.EqualFold(p.Title, f.Title) {
			continue
		}
		if f.Type != "" && p.Type != f.Type {
			continue
		}
		if f.Maturity != "" && p.Maturity != f.Maturity {
			continue
		}
		if f.Alias != "" && !containsFold(p.Aliases, f.Alias) {
			continue
		}
		if f.Tag != "" {
			tags, _ := d.tagsFor(p.Path)
			if !containsFold(tags, f.Tag) {
				continue
			}
		}
		if f.LinkedFrom != "" {
			edges, _ := d.OutgoingLinks(f.LinkedFrom)
			if !edgeTargetsContain(edges, p.Path) {
				continue
			}
		}
		if f.LinksTo != "" {
			// "links to title" → resolve title to path, then look up incoming.
			// Resolver has the truth; rather than embed it here, we settle for:
			// match if p has any outgoing edge whose target equals f.LinksTo
			// (case-insensitive).
			edges, _ := d.OutgoingLinks(p.Path)
			matched := false
			for _, e := range edges {
				if strings.EqualFold(e.Target, f.LinksTo) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if f.Orphan {
			incoming, _ := d.IncomingLinks(p.Path)
			real := 0
			for _, e := range incoming {
				if filepath.Base(e.Source) != "index.md" {
					real++
				}
			}
			if real > 0 {
				continue
			}
		}
		if f.Stale {
			if !d.isPageStale(v, p) {
				continue
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// Vaultish is the slice of vault.Vault that Query needs. We deliberately
// don't import vault here to avoid an import cycle with core.
type Vaultish interface {
	RootDir() string
}

// isPageStale checks whether any source file's mtime exceeds the page's
// updated date (end-of-day). Mirror of lint's stale check; duplicated
// here so manifest stays self-contained for the query path.
func (d *DB) isPageStale(v Vaultish, p PageInfo) bool {
	if p.Updated == "" || len(p.Sources) == 0 {
		return false
	}
	updated, err := time.Parse("2006-01-02", p.Updated)
	if err != nil {
		return false
	}
	updatedEOD := updated.Add(24 * time.Hour)
	for _, src := range p.Sources {
		mtimeStr, err := d.MTime(src)
		if err != nil {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, mtimeStr)
		if err != nil {
			continue
		}
		if t.After(updatedEOD) {
			return true
		}
	}
	_ = v
	return false
}

// tagsFor returns the JSON-decoded tags for one path.
func (d *DB) tagsFor(path string) ([]string, error) {
	var raw string
	err := d.sql.QueryRow(`SELECT tags FROM frontmatter WHERE path = ?`, path).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("tags lookup %s: %w", path, err)
	}
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out, nil
}

func containsFold(slice []string, want string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

func edgeTargetsContain(edges []WikilinkEdge, path string) bool {
	for _, e := range edges {
		if e.Resolved == path || strings.EqualFold(e.Target, filepath.Base(path)) {
			return true
		}
	}
	return false
}

// ChangedSinceLastIngest returns paths that have been scanned (or
// updated) since the most recent completed ingest finalized. Used by
// `sparks affected` to decide which collections need regeneration.
//
// If no ingest has ever finalized, every tracked file is considered
// changed (cold-start case — first ingest will rebuild everything).
func (d *DB) ChangedSinceLastIngest() ([]string, error) {
	var lastFinalized string
	err := d.sql.QueryRow(
		`SELECT COALESCE(MAX(finalized_at), '') FROM ingests WHERE status = 'completed'`,
	).Scan(&lastFinalized)
	if err != nil {
		return nil, err
	}

	q := `SELECT path FROM files WHERE deleted = 0`
	args := []interface{}{}
	if lastFinalized != "" {
		q += ` AND scan_time > ?`
		args = append(args, lastFinalized)
	}

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
