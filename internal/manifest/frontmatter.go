package manifest

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// FrontmatterRecord is the parsed frontmatter of a wiki page mirrored into
// SQLite for fast queries (count by type, find by tag, etc.).
type FrontmatterRecord struct {
	Path     string
	Title    string
	Type     string
	Maturity string
	Tags     []string
	Aliases  []string
	Sources  []string
	Created  string
	Updated  string
}

// UpsertFrontmatter writes one frontmatter record. Slices are JSON-encoded
// so SQLite stays portable; we trade type-richness for simplicity.
func (d *DB) UpsertFrontmatter(fr FrontmatterRecord) error {
	tagsJSON, _ := json.Marshal(stringsOrEmpty(fr.Tags))
	aliasesJSON, _ := json.Marshal(stringsOrEmpty(fr.Aliases))
	sourcesJSON, _ := json.Marshal(stringsOrEmpty(fr.Sources))
	_, err := d.sql.Exec(
		`INSERT INTO frontmatter(path, title, type, maturity, tags, aliases, sources, created, updated)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		   title = excluded.title,
		   type = excluded.type,
		   maturity = excluded.maturity,
		   tags = excluded.tags,
		   aliases = excluded.aliases,
		   sources = excluded.sources,
		   created = excluded.created,
		   updated = excluded.updated`,
		fr.Path, fr.Title, fr.Type, fr.Maturity,
		string(tagsJSON), string(aliasesJSON), string(sourcesJSON),
		fr.Created, fr.Updated,
	)
	if err != nil {
		return fmt.Errorf("upsert frontmatter %s: %w", fr.Path, err)
	}
	return nil
}

// DeleteFrontmatter removes a frontmatter row. Called when a file is
// no longer markdown or is removed from the vault.
func (d *DB) DeleteFrontmatter(path string) error {
	_, err := d.sql.Exec(`DELETE FROM frontmatter WHERE path = ?`, path)
	return err
}

// CountPagesByType returns counts grouped by frontmatter.type. Excludes
// rows where the parent file is marked deleted.
func (d *DB) CountPagesByType() (map[string]int, error) {
	rows, err := d.sql.Query(
		`SELECT f.type, COUNT(*) FROM frontmatter f
		   JOIN files fl ON f.path = fl.path
		  WHERE fl.deleted = 0 AND f.type IS NOT NULL AND f.type != ''
		  GROUP BY f.type`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var typ string
		var n int
		if err := rows.Scan(&typ, &n); err != nil {
			return nil, err
		}
		out[typ] = n
	}
	return out, rows.Err()
}

// FileCounts returns total tracked files, files changed since last ingest,
// and files marked deleted.
func (d *DB) FileCounts() (total, changed, deleted int, err error) {
	row := d.sql.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN ingested = 0 AND deleted = 0 THEN 1 ELSE 0 END),
			SUM(CASE WHEN deleted = 1 THEN 1 ELSE 0 END)
		FROM files
	`)
	var c, d_ sql.NullInt64
	if err = row.Scan(&total, &c, &d_); err != nil {
		return 0, 0, 0, err
	}
	if c.Valid {
		changed = int(c.Int64)
	}
	if d_.Valid {
		deleted = int(d_.Int64)
	}
	return total, changed, deleted, nil
}

func stringsOrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
