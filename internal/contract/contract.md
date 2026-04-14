# Sparks: Runtime Contracts

This document specifies the contracts that Sparks, agents, and humans all rely on. It is the reference for anyone building on, integrating with, or writing content inside a Sparks-managed vault. Content here is portable: it becomes the basis for agent instruction files (`AGENTS.md`, `GEMINI.md`, etc.) inside any Sparks vault.

## What Sparks Is

A single Go binary that maintains the mechanical integrity of a personal-scale knowledge base. Agents do content work; Sparks does plumbing. No editor, no UI, no server. Git handles sync; any markdown viewer (Obsidian, VS Code, plain `less`) handles browsing.

## The Three-Layer Model

```
┌─────────────────────────────────────────────┐
│ raw/       ← append-only human capture      │
├─────────────────────────────────────────────┤
│ wiki/      ← agent-maintained derived view  │
├─────────────────────────────────────────────┤
│ sparks.db  ← CLI-maintained manifest        │
└─────────────────────────────────────────────┘
```

**Raw** is the source of truth. **Wiki** is a derived view. **Manifest** tracks state for incremental operations. All three are plain files on disk (the manifest is a single SQLite file), versioned by git.

## Ownership Contract

| Thing                          | Human                    | Agent          | CLI                        |
|--------------------------------|--------------------------|----------------|----------------------------|
| `inbox.md` write               | yes                      | no             | no                         |
| `inbox.md` clear               | no                       | no             | yes (`ingest --finalize`)  |
| `raw/` new files               | yes                      | no             | no                         |
| `raw/` existing file edits     | typos, wikilinks only    | no             | no                         |
| `raw/inbox/YYYY-MM-DD.md`      | no                       | no             | yes                        |
| `raw/archive/` moves           | yes                      | no             | no                         |
| `wiki/entities/` etc.          | read                     | write          | validate                   |
| `wiki/collections/Tasks.md`    | read                     | write          | `done`, `tasks add`        |
| Other `wiki/collections/*`     | read                     | read           | regenerate                 |
| `wiki/index.md`                | read                     | append         | rebuild (`sparks index`)   |
| `wiki/log.md`                  | read                     | append         | no                         |
| `sparks.db`                    | no                       | no             | yes                        |
| `sparks.toml`                  | yes                      | no             | create on `init`           |
| Git commits                    | yes                      | no*            | yes (if `auto_commit`)     |

*Agents should not make git commits themselves. Sparks handles commits deterministically on `ingest --finalize` and `collections regen`.

## The Append-Only Raw Contract

In V1, `raw/` is append-only:

- Humans add new files. Agents never modify `raw/`.
- To add new context to an existing topic, drop an entry in `inbox.md` referencing the old note. Ingest reconciles it as a `## Revision` section on the appropriate wiki page.
- To retire a note, move it to `raw/archive/`. Do not delete.
- Light human edits (typos, adding `[[wikilinks]]`) are tolerated but discouraged. They can trigger `stale-pages` lint warnings.

**Why append-only:** the manifest, revision tracking, source citations, and stale-page detection all assume raw file stability. Editing raw content creates ambiguity about whether a wiki page should be re-derived. V1 sidesteps this with discipline. Future versions may introduce change-aware raw modes.

## Page Types (hardcoded)

Every wiki page has YAML frontmatter:

```yaml
---
title: string        # required
type: enum           # required: entity | concept | summary | synthesis | collection
maturity: enum       # required: seed | working | stable | historical
tags: [string]       # optional, no # prefix
aliases: [string]    # optional
sources: [string]    # required: vault-relative paths to raw/
created: YYYY-MM-DD  # required
updated: YYYY-MM-DD  # required
---
```

**Types:**

- **entity** — a person, tool, project, or company: `wiki/entities/BigQuery.md`
- **concept** — a theme, pattern, or technique: `wiki/concepts/Zero-Friction Capture.md`
- **summary** — a distilled source summary: `wiki/summaries/Karpathy LLM Wiki.md`
- **synthesis** — cross-cutting analysis or comparison: `wiki/synthesis/AI Agent Landscape.md`
- **collection** — auto-generated index of a content type: `wiki/collections/Quotes.md`

**Maturity:**

- **seed** — stub or initial capture, needs expansion
- **working** — actively developed, may change
- **stable** — well-sourced, reviewed, unlikely to change
- **historical** — preserved for context, no longer current

**Aliases** are alternate names, abbreviations, or prior names. Agents must check existing `aliases:` fields before creating a new page — dedup is identity-based, not title-based (e.g., `Claude Code for BigQuery` is an alias of `Cascade`).

Use `[[wikilinks]]` for internal links. Tags are plain strings (no `#`).

## Collections (hardcoded)

| Collection    | Default source                        | Overridable | Regen | Edit  |
|---------------|---------------------------------------|-------------|-------|-------|
| Quotes        | `raw/quotes/**/*.md`                  | yes         | CLI   | no    |
| Bookmarks     | `raw/weblinks/**/*.md`                | yes         | CLI   | no    |
| Books         | quote attributions + `book:` in inbox | no          | CLI   | no    |
| Reading List  | `to-read:` in inbox                   | no          | CLI   | no    |
| Media         | `raw/media/**/*.md`                   | yes         | CLI   | no    |
| Ideas         | `raw/ideas/**/*.md` + inbox hints     | yes (glob)  | CLI   | no    |
| Projects      | wiki entities with `project` tag      | no          | CLI   | no    |
| Tasks         | live edited                           | —           | no    | agent |

Source globs for Quotes, Bookmarks, Media, and Ideas can be overridden in `sparks.toml` under `[collections.<name>]`. Collection types, extractors, and output filenames are hardcoded.

**Tasks is special.** It is not regenerated. Agents edit it directly (via `sparks tasks add` to append, `sparks done` to complete). The raw `raw/sparks/tasks/` directory is historical capture only — the Tasks collection is the source of truth for task status.

Collection regeneration is deterministic. Never edit a regenerated collection page by hand; changes will be overwritten on next `sparks collections regen`.

## The Ingest Contract

`sparks ingest --prepare` returns a structured manifest (JSON). The agent's job:

1. For each entry, identify relevant wiki pages to create or update.
2. Use the **capture date** (not today) as `created:` for new pages. Use today as `updated:`.
3. Check existing pages' `aliases:` for dedup before creating. Prefer updating over creating.
4. If an existing wiki page contradicts a new entry, append a `## Revision` section noting the contradiction and both sources. Never silently overwrite.
5. Set `maturity: seed` on new pages unless clearly well-sourced.
6. Add a one-line entry to `wiki/index.md` for each new page.
7. Append an entry to `wiki/log.md`: `## [YYYY-MM-DD] ingest | summary of changes`.
8. For inbox TODOs / checkboxes, call `sparks tasks add --section "[[Project]]" --text "..."` to append under the right project heading.
9. Call `sparks ingest --finalize` when done.

**Process no more than 10–15 inbox entries per session** to stay within reasonable token budgets. If more remain, the agent should stop and say so; the human re-runs.

## The Query Contract

`sparks query` is for structured lookup only:

- by title / alias
- by type, maturity, tag
- by frontmatter field values
- by link graph (`--linked-from`, `--links-to`)
- by state (`--stale`, `--orphan`, `--thin`)

Semantic search, keyword search over content, and synthesis are **agent work**. The CLI does not understand content.

## The Lint Contract

`sparks lint` is deterministic. It reports:

- orphan pages, broken wikilinks, missing or invalid frontmatter, thin pages, stale pages (wiki older than source), dead source paths, duplicate aliases.

`--fix` applies only safe fixes (remove broken-link markers, normalize tag formatting, bump stale `updated:` dates). Semantic fixes (rewriting thin pages, resolving contradictions) are agent work.

## The Git Contract

- Sparks commits using the vault's configured git identity. It does not set, override, or default a commit email — if you want vault commits to use a different identity than your usual one, run `git config --local user.email "..."` in the vault like any other repo.
- Sparks commits only on `ingest --finalize` and `collections regen` (when `auto_commit = true`).
- Sparks never force-pushes, never resets, never amends.
- Humans are free to commit themselves. Agents should not — the CLI owns commit atomicity.

## Agent Integration

Drop a short instruction file into your vault for each agent you use:

**`AGENTS.md`** (or `CLAUDE.md`, `GEMINI.md` — same content, different filename):

```markdown
# Vault managed by Sparks

Run `sparks status` for an overview. All plumbing is handled by the CLI;
your job is semantic — deciding what pages to create, writing content,
synthesizing across pages.

## Ingest (process inbox.md)

1. `sparks ingest --prepare --json` — read the output.
2. For each entry: create or update wiki pages per the Sparks page-type contract.
   - Use the entry's `capture_date` as `created:`.
   - Check `aliases:` on existing pages before creating.
   - If contradicting existing content, add a `## Revision` section — never silently overwrite.
   - Set `maturity: seed` on new pages unless clearly well-sourced.
3. For any TODOs in the entry, call `sparks tasks add --section "[[Project]]" --text "..."`.
4. Update `wiki/index.md` (one line per new page).
5. Append to `wiki/log.md`: `## [YYYY-MM-DD] ingest | <summary>`.
6. `sparks ingest --finalize`.

## Query

1. `sparks query` for structured lookup (by title, tag, type, link graph).
2. Read the matching wiki pages.
3. Synthesize the answer with `[[wikilink]]` citations.

## Task management

- `sparks done "<task description>"` — mark a task complete (fuzzy match).
- `sparks tasks add --section "[[Project]]" --text "<task>"` — add a new task.

## Health checks

- `sparks lint` before declaring work done.
- `sparks fmt --check` to validate frontmatter.

## What never to do

- Never edit `raw/` (except the human writing into `inbox.md`).
- Never edit `sparks.db`.
- Never edit a regenerated collection page (everything except Tasks).
- Never make git commits — Sparks handles them on `ingest --finalize`.
- Never delete notes; move to `raw/archive/` instead.
```

The same content works across all agents. The protocol is the CLI, not the instruction prose.

## Preserved Behaviors (from Karpathy-style wikis)

Sparks v1 explicitly preserves the behaviors that make the Karpathy LLM-wiki pattern work:

- Immutable raw capture + derived wiki.
- Revision blocks instead of silent overwrites.
- Frontmatter-first page identity with aliases for dedup.
- Stale-page detection via manifest timestamps.
- Incremental collection regeneration (only what changed).
- Chronological operation log for audit.
- Content-addressable file tracking via SHA-256 manifest.
- Per-entry capture-date preservation (distinct from ingest date).
- Processing budget per session (10–15 entries) to keep agent output coherent.

These are not implementation details — they are load-bearing invariants. Any reimplementation of Sparks must preserve them.
