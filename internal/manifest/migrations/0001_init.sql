-- Sparks manifest schema, version 1.
--
-- Files: every tracked file in the vault, content-addressable via SHA-256.
-- Frontmatter: parsed YAML for wiki pages, mirrored from files for fast queries.
-- Wikilinks: graph edges between wiki pages.
-- Ingests: history of inbox-processing sessions.
-- Schema_version: single-row migrations marker.

CREATE TABLE IF NOT EXISTS files (
    path        TEXT PRIMARY KEY,
    hash        TEXT NOT NULL,
    size        INTEGER NOT NULL,
    mtime       TEXT NOT NULL,         -- RFC3339Nano
    scan_time   TEXT NOT NULL,         -- RFC3339Nano
    ingested    INTEGER NOT NULL DEFAULT 0,
    deleted     INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_files_deleted ON files(deleted);
CREATE INDEX IF NOT EXISTS idx_files_ingested ON files(ingested);

CREATE TABLE IF NOT EXISTS frontmatter (
    path        TEXT PRIMARY KEY REFERENCES files(path) ON DELETE CASCADE,
    title       TEXT,
    type        TEXT,
    maturity    TEXT,
    tags        TEXT NOT NULL DEFAULT '[]',     -- JSON array
    aliases     TEXT NOT NULL DEFAULT '[]',     -- JSON array
    sources     TEXT NOT NULL DEFAULT '[]',     -- JSON array
    created     TEXT,
    updated     TEXT
);

CREATE INDEX IF NOT EXISTS idx_frontmatter_type ON frontmatter(type);
CREATE INDEX IF NOT EXISTS idx_frontmatter_maturity ON frontmatter(maturity);
CREATE INDEX IF NOT EXISTS idx_frontmatter_title ON frontmatter(title);

CREATE TABLE IF NOT EXISTS wikilinks (
    source      TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    target      TEXT NOT NULL,
    resolved    TEXT,
    PRIMARY KEY (source, target)
);

CREATE INDEX IF NOT EXISTS idx_wikilinks_target ON wikilinks(target);

CREATE TABLE IF NOT EXISTS ingests (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at      TEXT NOT NULL,
    finalized_at    TEXT,
    status          TEXT NOT NULL DEFAULT 'in_progress', -- in_progress | completed | aborted
    entry_count     INTEGER,
    commit_sha      TEXT
);

CREATE INDEX IF NOT EXISTS idx_ingests_status ON ingests(status);

CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL
);

INSERT OR IGNORE INTO schema_version(version, applied_at) VALUES (1, datetime('now'));
