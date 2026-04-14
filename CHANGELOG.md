# Changelog

All notable changes to Sparks. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and Sparks adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) starting at v0.1.0.

## [Unreleased]

## [0.1.0] - 2026-04-14

**Pre-alpha first release.** The internal packages are tested (~75% line coverage, race-clean across 12 packages on a CI matrix of macOS/Linux/Windows). The end-to-end agent-driven workflow has been dogfooded on the maintainer's vault but is not 100% manually tested. Expect rough edges. [Open issues](https://github.com/yogirk/sparks/issues) freely — that's the fastest way to harden the v0.x line.

Sparks is a single Go binary that maintains the mechanical integrity of an agent-driven personal knowledge base.

### Added

- `sparks init [--agent X]` — initialize a vault, optionally drop a per-agent instruction file (CLAUDE.md, AGENTS.md, GEMINI.md).
- `sparks scan` — incremental SQLite manifest with WAL + busy_timeout, schema migrations, content-hash + frontmatter + wikilink graph.
- `sparks status` — vault overview with page counts by type, inbox pending, manifest stats.
- `sparks ingest --prepare/--finalize/--abort` — two-phase inbox processing with concurrent-ingest lock.
- `sparks done <query>` — fuzzy-match + toggle a task complete.
- `sparks tasks add --section X --text Y` — append to the live Tasks page.
- `sparks lint` — eight deterministic checks: orphans, broken-links, missing-frontmatter, invalid-frontmatter, thin-pages, stale-pages, dead-sources, duplicate-aliases.
- `sparks fmt` — frontmatter validation across wiki pages.
- `sparks collections regen` — seven regenerated collections (Quotes, Bookmarks, Books, ReadingList, Media, Ideas, Projects).
- `sparks index` — rebuild `wiki/index.md` preserving agent-authored descriptions.
- `sparks query` — structured lookup over the manifest by title/alias/tag/type/maturity/link-graph/state.
- `sparks affected` — which collections need regeneration since the last completed ingest.
- `sparks describe` — print the canonical agent-runtime contract embedded in the binary.
- `sparks serve` — MCP server over stdio exposing all 11 operations as native tools.

### Architecture

- Thin-adapter discipline enforced by `cmd/sparks/arch_test.go`: no SQLite, YAML, TOML, MCP, or os/exec imports in the CLI layer; handlers capped at 50 lines. Same discipline in `internal/mcp/`.
- Resolver matches wikilinks by title → alias → filename basename (Obsidian convention).
- Pure-Go SQLite via `modernc.org/sqlite` — no CGo, single static binary.
- Cross-platform: paths stored with forward slashes for vault portability across macOS, Linux, Windows.
