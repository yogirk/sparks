# Sparks CLI — V1 Spec

## Overview

`sparks` is a Go CLI that provides the mechanical operational layer for an agent-maintained personal knowledge base. It handles everything that doesn't require language understanding: vault discovery, manifest tracking, frontmatter parsing, inbox splitting, collection regeneration, link-graph analysis, and MCP serving.

Agents call `sparks` for plumbing, then do semantic work themselves (content creation, synthesis, categorization).

## V1 Scope

**In scope:**

- Hardcoded Karpathy-shape KB: `entity / concept / summary / synthesis / collection` page types with fixed frontmatter schema.
- Append-only raw sources. Agent-owned mutable wiki layer.
- Personal-scale vault: single user, local filesystem, SQLite manifest.
- Git as persistence/sync.
- MCP stdio server for agent integration.
- Commands: `init`, `scan`, `affected`, `ingest --prepare/--finalize`, `done`, `tasks add`, `lint`, `fmt`, `collections regen`, `index`, `serve`, `status`, `query`.

**Out of scope (V1):**

- Custom page schemas, maturity values, or frontmatter fields.
- Document ingestion (PDF / DOCX / HTML conversion).
- Multi-user collaboration or review flows.
- Mutable raw sources.
- Semantic / vector / full-text search.
- Capture UX beyond `inbox.md`.
- Web UI or editor.
- Remote sync beyond git.

## Vault Assumptions

Every Sparks vault has this structure (paths are fixed, not configurable in V1):

```
<vault>/
├── inbox.md                  # capture interface (entries separated by ---)
├── raw/                      # immutable raw sources
│   ├── inbox/                # archived inbox items (YYYY-MM-DD.md)
│   ├── archive/              # retired notes (moved, not deleted)
│   └── <user-defined>/       # freely organized by the user
├── wiki/                     # agent-maintained knowledge base
│   ├── index.md              # content catalog
│   ├── log.md                # operation log
│   ├── entities/
│   ├── concepts/
│   ├── summaries/
│   ├── synthesis/
│   └── collections/          # auto-generated indexes
├── sparks.db                 # SQLite manifest
└── sparks.toml               # CLI config + vault root marker
```

`sparks.toml` marks the vault root and holds configuration:

```toml
[vault]
name = "my-vault"

[raw]
mode = "append-only"   # V1 supports only this mode

[git]
auto_commit = true     # commit after ingest --finalize and collections regen
# Commit identity follows your git config. Set a vault-local identity with
# `git config --local user.email "..."` if you want vault commits to use a
# different name/email than your usual git identity.

# Collection source globs (optional — defaults shown).
# Collection types, extractor logic, and output filenames are hardcoded;
# only the input globs are user-overridable. Omit any section to use the default.
[collections.quotes]
glob = "raw/quotes/**/*.md"

[collections.bookmarks]
glob = "raw/weblinks/**/*.md"

[collections.media]
glob = "raw/media/**/*.md"

[collections.ideas]
glob = "raw/ideas/**/*.md"

# Books, Reading List, Projects, and Tasks derive from other sources
# (inbox markers, wiki frontmatter) and are not glob-configurable.
```

Page types, maturity values, frontmatter schema, collection types, and extractor logic are **hardcoded in the binary**. The only user-configurable aspect of collections is their **source glob** (via `[collections.*]` above).

## Frontmatter Schema (hardcoded)

```yaml
---
title: string        # required
type: enum           # required: entity | concept | summary | synthesis | collection
maturity: enum       # required: seed | working | stable | historical
tags: [string]       # optional, no # prefix
aliases: [string]    # optional — alternate names for dedup
sources: [string]    # required — vault-relative paths to raw/ files
created: YYYY-MM-DD  # required
updated: YYYY-MM-DD  # required
---
```

## Collections (hardcoded types, overridable paths)

Sparks ships with these collection types, each with fixed extractors and output filenames. Source globs have defaults and are overridable in `sparks.toml`:

| Collection    | Default source                        | Overridable | Behavior         |
|---------------|---------------------------------------|-------------|------------------|
| Quotes        | `raw/quotes/**/*.md`                  | yes         | regenerated      |
| Bookmarks     | `raw/weblinks/**/*.md`                | yes         | regenerated      |
| Books         | quote attributions + `book:` in inbox | no          | regenerated      |
| Reading List  | `to-read:` in inbox                   | no          | regenerated      |
| Media         | `raw/media/**/*.md`                   | yes         | regenerated      |
| Ideas         | `raw/ideas/**/*.md` + inbox hints     | yes (glob)  | regenerated      |
| Projects      | wiki entities with `project` tag      | no          | regenerated      |
| Tasks         | edited in place                       | —           | live (no regen)  |

**Tasks exception.** The Tasks collection is a live editable page. It is never regenerated. During ingest, new tasks extracted from inbox (`- [ ]` or `TODO:` lines) are *appended* to `wiki/collections/Tasks.md` under the appropriate project heading. Completed tasks are toggled via `sparks done`. The raw `raw/sparks/tasks/` directory is historical capture only.

Paths are globs over `raw/`. Users can organize raw files freely within those subtrees.

## Commands

### `sparks init`

Initialize a vault. Creates `sparks.toml`, `sparks.db`, and the required directory structure if missing. Idempotent.

```
sparks init [path]
```

### `sparks scan`

Scan the vault and update the SQLite manifest. Records file paths, SHA-256 hashes, frontmatter, and timestamps.

```
sparks scan [--verbose]
```

Output: summary of new / changed / deleted files since last scan.

### `sparks affected`

Report which collections need regeneration based on changes since last ingest.

```
sparks affected [--json]
```

Output (text): `Quotes,Bookmarks`, `all`, or `none`.
Output (JSON): `{"affected": ["Quotes"], "reason": {"Quotes": ["raw/sparks/quotes/new.md"]}}`

### `sparks ingest --prepare`

Parse `inbox.md`, split entries, produce a structured manifest for the agent.

```
sparks ingest --prepare [--json]
```

**Behavior:**

1. Read `inbox.md`.
2. Split entries by `---` (line-anchored).
3. Parse capture date from each entry (first line if `YYYY-MM-DD`, else today).
4. For each entry, compute a content hash and flag deterministic hints:
   - `to-read:` prefix → Reading List candidate
   - `book:` prefix → Books candidate
   - `- [ ]` or `TODO:` lines → Tasks candidates (with surrounding context)
   - plain URLs → Bookmarks candidates
   - lines beginning/ending with quote markers + attribution → Quotes candidates
5. Output structured data.

**Output (JSON):**

```json
{
  "entries": [
    {
      "id": 1,
      "capture_date": "2026-04-13",
      "content": "the raw entry text...",
      "hash": "abc123...",
      "hints": {
        "tasks": ["review spec", "ship v1"],
        "bookmarks": ["https://..."],
        "has_to_read": false,
        "has_book": false,
        "has_quote": false
      }
    }
  ],
  "total": 3,
  "affected_collections": ["Ideas", "Tasks"]
}
```

**Does NOT:** create wiki pages, decide categorization, write content, classify semantically. That's the agent's job. Hints are deterministic pattern matches, not semantic labels.

### `sparks ingest --finalize`

Post-processing after the agent has done semantic work.

```
sparks ingest --finalize [--message "ingest: 3 entries processed"]
```

**Behavior:**

1. Archive inbox entries to `raw/inbox/YYYY-MM-DD.md` (grouped by capture date, append if exists).
2. Clear `inbox.md` (preserve header block).
3. Run `sparks scan`.
4. Regenerate affected collections (skip Tasks).
5. Mark files `ingested=1` in manifest.
6. Git commit if `auto_commit = true`.

### `sparks done`

Mark a task complete in `wiki/collections/Tasks.md`.

```
sparks done "review spec"
```

Fuzzy match against open `- [ ]` items. If ambiguous, print candidates and exit non-zero.

### `sparks tasks add`

Append a task under a project heading in `wiki/collections/Tasks.md`.

```
sparks tasks add --section "[[Sparks]]" --text "Resolve open design questions"
```

Creates the section heading if missing. This is the helper agents use during ingest to surface TODOs from inbox into the live Tasks page.

### `sparks lint`

Analyze vault health. All checks are deterministic.

```
sparks lint [--fix] [--json]
```

**Checks:**

- `orphans` — wiki pages not linked from `index.md` or any other page.
- `broken-links` — `[[wikilinks]]` pointing to non-existent pages.
- `missing-frontmatter` — wiki pages without required fields.
- `invalid-frontmatter` — wrong types or unknown enum values.
- `thin-pages` — fewer than 3 sentences of content (excluding frontmatter).
- `stale-pages` — wiki `updated` older than source file mtime.
- `dead-sources` — `sources:` pointing to non-existent files.
- `duplicate-aliases` — multiple pages claiming the same alias.

`--fix` auto-fixes only what's safe (remove broken link markers, normalize tag formatting, bump stale `updated:` dates). Semantic fixes (rewriting thin pages, resolving contradictions) are agent work.

### `sparks fmt`

Validate and optionally fix frontmatter on wiki pages.

```
sparks fmt [file|glob] [--fix] [--check]
```

Validates required fields, enum values, tag formatting, `sources:` path existence, and date formats. `--check` exits non-zero on any issue (for CI/hooks).

### `sparks collections regen`

Regenerate collection pages from raw sources.

```
sparks collections regen [name...] [--all] [--dry-run]
```

Skips Tasks. Without arguments, regenerates only affected collections.

### `sparks index`

Rebuild `wiki/index.md` from the current wiki state.

```
sparks index [--dry-run]
```

Generates one line per page: link, description, source count. Preserves agent-authored descriptions when they already exist (see Open Questions).

### `sparks serve`

Run as an MCP server over stdio.

```
sparks serve
```

Exposes these MCP tools:

- `sparks_scan`
- `sparks_prepare_ingest`
- `sparks_finalize_ingest`
- `sparks_done`
- `sparks_tasks_add`
- `sparks_lint`
- `sparks_fmt`
- `sparks_affected`
- `sparks_query`
- `sparks_index`
- `sparks_status`

### `sparks query`

Structured lookup over the manifest. **Not semantic search.**

```
sparks query --title "Cascade"
sparks query --tag data-engineering --type entity
sparks query --alias "BQ"
sparks query --linked-from "wiki/synthesis/Ideas Landscape.md"
sparks query --links-to "Cascade"
sparks query --stale
```

Agents call `sparks query` to locate pages, then read the files themselves. Semantic synthesis is agent work.

### `sparks status`

Quick vault overview.

```
sparks status
```

Output:

```
vault: my-vault (~/Projects/notes/my-vault)
pages: 23 entities, 3 concepts, 0 summaries, 3 synthesis, 7 collections
inbox: 4 entries pending
manifest: 212 files tracked, 3 changed since last ingest
health: 94% (run sparks lint for details)
```

## Data Model

### SQLite Manifest (`sparks.db`)

```sql
CREATE TABLE files (
    path       TEXT PRIMARY KEY,
    hash       TEXT NOT NULL,          -- SHA-256 of content
    size       INTEGER NOT NULL,
    mtime      TEXT NOT NULL,          -- ISO 8601
    scan_time  TEXT NOT NULL,
    ingested   INTEGER DEFAULT 0,
    deleted    INTEGER DEFAULT 0
);

CREATE TABLE frontmatter (
    path       TEXT PRIMARY KEY REFERENCES files(path),
    title      TEXT,
    type       TEXT,
    maturity   TEXT,
    tags       TEXT,                   -- JSON array
    aliases    TEXT,                   -- JSON array
    sources    TEXT,                   -- JSON array
    created    TEXT,
    updated    TEXT
);

CREATE TABLE wikilinks (
    source     TEXT NOT NULL REFERENCES files(path),
    target     TEXT NOT NULL,
    resolved   TEXT,                   -- resolved path, NULL if broken
    PRIMARY KEY (source, target)
);

CREATE TABLE ingests (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at     TEXT NOT NULL,
    finalized_at   TEXT,
    entry_count    INTEGER,
    commit_sha     TEXT
);
```

## Package Structure

```
sparks/
├── cmd/sparks/
│   └── main.go              # cobra root
├── internal/
│   ├── vault/               # vault discovery, config parsing
│   ├── manifest/            # SQLite, scan, affected, query
│   ├── frontmatter/         # parse, validate, write
│   ├── inbox/               # parse, classify hints, archive
│   ├── collections/         # per-type extractors (hardcoded)
│   ├── lint/                # orphans, broken links, stale, thin, etc.
│   ├── wiki/                # wikilinks, index rebuild
│   ├── tasks/               # done, add
│   └── mcp/                 # stdio MCP server
├── sparks.toml.example
├── go.mod / go.sum
└── Makefile
```

## Dependencies (minimal)

- `github.com/spf13/cobra` — CLI framework
- `github.com/pelletier/go-toml/v2` — config
- `modernc.org/sqlite` — pure-Go SQLite
- `gopkg.in/yaml.v3` — frontmatter
- `github.com/sahilm/fuzzy` — fuzzy matching for `done`
- `github.com/mark3labs/mcp-go` — MCP server SDK

## Resolved Design Decisions

- **`sparks query` semantics:** exact match, prefix match, and frontmatter / link-graph filters only. No keyword or semantic search — that's agent work.
- **`sparks serve` transport:** stdio only in V1. HTTP sidecar deferred.
- **Collection extraction:** hardcoded in Go per collection type. V1 ships with 8 extractors; adding a collection type is a code change, not a config change.
- **`sparks watch` (fsnotify-driven):** deferred to V2.
- **Git integration:** the CLI handles commits when `auto_commit = true`. Agents and users can disable and commit themselves.
- **KB shape configurability:** page types, maturity values, frontmatter schema, collection types, and extractor logic are hardcoded. Only collection **source globs** are user-overridable, with sensible defaults.

## Open Questions

- [ ] Exact JSON schema for `sparks ingest --prepare` hints — what's worth classifying deterministically vs. leaving entirely to the agent.
- [ ] Should `sparks index` rebuild the one-line descriptions, or preserve agent-authored ones? (Leaning: preserve.)
- [ ] MCP tool naming convention — underscore vs dot (leaning `sparks_ingest_prepare` for MCP client compatibility).
- [ ] Whether `sparks done` should accept `--fuzzy` / `--exact` flags or just always fuzzy-match with disambiguation output.

## Agent Integration

See `sparks-contracts.md` for the runtime contracts and the reference template that agents (Claude Code, Codex, Gemini CLI, etc.) load into their instruction files.
