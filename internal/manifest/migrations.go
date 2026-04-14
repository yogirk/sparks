package manifest

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migration is a single forward-only schema change.
type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads embedded migration files. Filenames must follow
// 0001_init.sql, 0002_thing.sql, etc. Numbers are the version order.
func loadMigrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".sql")
		parts := strings.SplitN(base, "_", 2)
		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			return nil, fmt.Errorf("invalid migration filename %q: %w", e.Name(), err)
		}
		body, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		out = append(out, migration{version: version, name: base, sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// applyMigrations runs every migration with version > current.
// Migration 1 is special: it creates the schema_version table itself, so we
// run it unconditionally on a fresh DB.
func applyMigrations(db *sql.DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := currentSchemaVersion(db)
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
	}
	return nil
}

// currentSchemaVersion returns the highest applied version, or 0 if the
// schema_version table does not yet exist.
func currentSchemaVersion(db *sql.DB) (int, error) {
	var name string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version'`,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect schema: %w", err)
	}
	var v int
	err = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("read schema_version: %w", err)
	}
	return v, nil
}
