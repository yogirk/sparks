// Package manifest is the SQLite-backed view of vault state.
//
// It tracks every file's content hash, parsed frontmatter for wiki pages,
// the wikilink graph, and ingest history. Callers mutate the manifest by
// calling specific methods; direct SQL is intentionally not exposed so
// schema migrations stay safe.
package manifest

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB with vault-aware helpers.
type DB struct{ sql *sql.DB }

// Sentinel errors.
var (
	ErrNotFound = errors.New("manifest: record not found")
)

// Open opens (or creates) a SQLite manifest at path, enables WAL mode and a
// busy timeout for concurrent access, and applies any pending migrations.
//
// The DSN sets pragmas inline so they apply on every new connection in the
// pool, not just the first.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)",
		path,
	)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single connection is fine for our access pattern (CLI-driven, brief
	// lifetimes). Keeps WAL behavior predictable.
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := applyMigrations(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{sql: sqlDB}, nil
}

// Close closes the underlying SQLite connection.
func (d *DB) Close() error { return d.sql.Close() }

// FileRecord is one row from the files table.
type FileRecord struct {
	Path     string
	Hash     string
	Size     int64
	MTime    time.Time
	ScanTime time.Time
	Ingested bool
	Deleted  bool
}

// GetFile returns the existing record for path, or ErrNotFound.
func (d *DB) GetFile(path string) (FileRecord, error) {
	row := d.sql.QueryRow(
		`SELECT path, hash, size, mtime, scan_time, ingested, deleted
		   FROM files WHERE path = ?`,
		path,
	)
	var fr FileRecord
	var mtimeStr, scanTimeStr string
	var ingested, deleted int
	err := row.Scan(&fr.Path, &fr.Hash, &fr.Size, &mtimeStr, &scanTimeStr, &ingested, &deleted)
	if err == sql.ErrNoRows {
		return FileRecord{}, ErrNotFound
	}
	if err != nil {
		return FileRecord{}, err
	}
	fr.MTime, _ = time.Parse(time.RFC3339Nano, mtimeStr)
	fr.ScanTime, _ = time.Parse(time.RFC3339Nano, scanTimeStr)
	fr.Ingested = ingested == 1
	fr.Deleted = deleted == 1
	return fr, nil
}

// UpsertFile inserts or updates a file record. Resets deleted=0 because a
// re-discovered file is no longer deleted.
func (d *DB) UpsertFile(fr FileRecord) error {
	_, err := d.sql.Exec(
		`INSERT INTO files(path, hash, size, mtime, scan_time, ingested, deleted)
		 VALUES(?, ?, ?, ?, ?, ?, 0)
		 ON CONFLICT(path) DO UPDATE SET
		   hash = excluded.hash,
		   size = excluded.size,
		   mtime = excluded.mtime,
		   scan_time = excluded.scan_time,
		   deleted = 0`,
		fr.Path, fr.Hash, fr.Size,
		fr.MTime.Format(time.RFC3339Nano),
		fr.ScanTime.Format(time.RFC3339Nano),
		boolToInt(fr.Ingested),
	)
	if err != nil {
		return fmt.Errorf("upsert file %s: %w", fr.Path, err)
	}
	return nil
}

// MarkDeleted flips deleted=1 for paths not seen during a scan. We keep the
// row so historical references don't break; lint --fix can later prune.
func (d *DB) MarkDeleted(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`UPDATE files SET deleted = 1 WHERE path = ?`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, p := range paths {
		if _, err := stmt.Exec(p); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("mark deleted %s: %w", p, err)
		}
	}
	return tx.Commit()
}

// AllPaths returns every non-deleted file path. Used during scan to compute
// the set of files that disappeared.
func (d *DB) AllPaths() ([]string, error) {
	rows, err := d.sql.Query(`SELECT path FROM files WHERE deleted = 0`)
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

// SchemaVersion returns the current applied schema version. Useful for
// status output and tests.
func (d *DB) SchemaVersion() (int, error) {
	return currentSchemaVersion(d.sql)
}

// Raw returns the underlying *sql.DB for queries that don't fit a typed
// helper. Use sparingly; prefer to add a typed method on DB. The escape
// hatch exists so packages like collections/projects can run a single
// custom join without bloating the manifest API surface.
func (d *DB) Raw() *sql.DB { return d.sql }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
