package manifest

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// WikilinkEdge is one source→target wikilink edge. Resolved is empty if
// the target didn't match any known page (case-insensitive title or
// alias). Callers distinguish broken links by Resolved == "".
type WikilinkEdge struct {
	Source   string
	Target   string
	Resolved string
}

// ReplaceLinks clears all wikilinks for source and inserts the new set
// in a single transaction. Wikilink relationships are derived from file
// bodies; they rebuild from scratch on every scan.
func (d *DB) ReplaceLinks(source string, edges []WikilinkEdge) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM wikilinks WHERE source = ?`, source); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear links: %w", err)
	}
	if len(edges) > 0 {
		stmt, err := tx.Prepare(
			`INSERT INTO wikilinks(source, target, resolved) VALUES(?, ?, ?)
			 ON CONFLICT(source, target) DO UPDATE SET resolved = excluded.resolved`,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		defer stmt.Close()
		for _, e := range edges {
			var resolved interface{}
			if e.Resolved != "" {
				resolved = e.Resolved
			} else {
				resolved = nil
			}
			if _, err := stmt.Exec(e.Source, e.Target, resolved); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("insert link %s->%s: %w", source, e.Target, err)
			}
		}
	}
	return tx.Commit()
}

// OutgoingLinks returns all edges originating from source.
func (d *DB) OutgoingLinks(source string) ([]WikilinkEdge, error) {
	return d.queryLinks(`SELECT source, target, COALESCE(resolved, '') FROM wikilinks WHERE source = ?`, source)
}

// IncomingLinks returns all edges whose resolved path equals target. Used
// by orphan detection — a page with no incoming links (other than index)
// is an orphan.
func (d *DB) IncomingLinks(target string) ([]WikilinkEdge, error) {
	return d.queryLinks(`SELECT source, target, COALESCE(resolved, '') FROM wikilinks WHERE resolved = ?`, target)
}

// BrokenLinks returns every edge whose resolved is NULL (i.e., the
// target didn't match any existing page at scan time).
func (d *DB) BrokenLinks() ([]WikilinkEdge, error) {
	return d.queryLinks(`SELECT source, target, '' FROM wikilinks WHERE resolved IS NULL OR resolved = ''`)
}

// ListPages returns every non-deleted wiki page with its frontmatter
// essentials, for building a resolver or iterating lint checks.
func (d *DB) ListPages() ([]PageInfo, error) {
	rows, err := d.sql.Query(`
		SELECT f.path, f.title, f.type, f.maturity, f.aliases, f.sources,
		       COALESCE(f.created, ''), COALESCE(f.updated, '')
		  FROM frontmatter f
		  JOIN files fl ON fl.path = f.path
		 WHERE fl.deleted = 0 AND f.path LIKE 'wiki/%'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageInfo
	for rows.Next() {
		var (
			p                                PageInfo
			aliasesJSON, sourcesJSON         string
		)
		if err := rows.Scan(
			&p.Path, &p.Title, &p.Type, &p.Maturity,
			&aliasesJSON, &sourcesJSON, &p.Created, &p.Updated,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(aliasesJSON), &p.Aliases)
		_ = json.Unmarshal([]byte(sourcesJSON), &p.Sources)
		out = append(out, p)
	}
	return out, rows.Err()
}

// PageInfo is the denormalized page metadata used by lint and graph work.
type PageInfo struct {
	Path     string
	Title    string
	Type     string
	Maturity string
	Aliases  []string
	Sources  []string
	Created  string
	Updated  string
}

func (d *DB) queryLinks(q string, args ...interface{}) ([]WikilinkEdge, error) {
	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WikilinkEdge
	for rows.Next() {
		var e WikilinkEdge
		if err := rows.Scan(&e.Source, &e.Target, &e.Resolved); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MTime returns the filesystem mtime recorded for a file, or zero if the
// file is not tracked. Useful for stale-page detection which compares
// wiki `updated` against max(source file mtime).
func (d *DB) MTime(path string) (string, error) {
	var m string
	err := d.sql.QueryRow(`SELECT mtime FROM files WHERE path = ? AND deleted = 0`, path).Scan(&m)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return m, err
}

// WikiFilesWithoutFrontmatter returns paths of wiki/*.md files tracked in
// the manifest that have no frontmatter row. These are pages the agent
// wrote without a YAML header — lint flags them as missing-frontmatter
// even when no fields are set.
func (d *DB) WikiFilesWithoutFrontmatter() ([]string, error) {
	rows, err := d.sql.Query(`
		SELECT f.path FROM files f
		LEFT JOIN frontmatter fm ON fm.path = f.path
		WHERE f.deleted = 0
		  AND f.path LIKE 'wiki/%'
		  AND f.path LIKE '%.md'
		  AND (fm.path IS NULL OR (fm.title IS NULL AND fm.type IS NULL))
	`)
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
