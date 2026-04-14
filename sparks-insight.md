# Sparks: The Insight

## What Sparks Is

**Sparks is a knowledge base runtime for AI agents.** A single Go binary that handles the mechanical work of maintaining a personal-scale knowledge base, so agents can focus on content. Any agent — Claude Code, Codex, Gemini CLI, or whatever ships next — operates the same vault through one CLI.

Not a notes app. Not an editor. Not a sync tool. A runtime.

## The Problem

Personal KB tools (Obsidian, Notion, Roam) are designed for human interaction. But the emerging pattern is: **humans capture, AI agents maintain**. The human drops raw thoughts in an inbox. An agent processes, categorizes, links, synthesizes. The human reads the output in a familiar markdown viewer.

This works — but when the operational logic lives as prose instructions inside a single agent's config file (CLAUDE.md, AGENTS.md, GEMINI.md), the agent ends up doing two fundamentally different kinds of work:

1. **Mechanical** — splitting inbox entries, hashing files, parsing frontmatter, detecting orphans, regenerating collection indexes, archiving, git commits.
2. **Semantic** — understanding content, deciding what pages to create, writing content, synthesizing answers, spotting contradictions.

The mechanical work is deterministic. It doesn't need an LLM. When it's trapped in prose:

- It's coupled to one agent.
- It's re-interpreted every run (fragile, non-deterministic).
- It burns tokens on file hashing and separator-splitting.
- It's not testable or version-controlled as logic.

## The Insight

Extract the mechanical layer into a Go CLI. The binary becomes the runtime. Any agent calls the CLI for plumbing, then does semantic work itself.

Agent instructions collapse from 200 lines of protocol to:

```
1. sparks ingest --prepare
2. Read its output. Create/update wiki pages for each entry.
3. sparks ingest --finalize
```

## What Falls Out

- `sparks lint` runs instantly — orphans, broken links, thin pages are graph traversal on frontmatter.
- `sparks collections regen` is deterministic — scan, group, format. No LLM.
- `sparks serve` exposes MCP tools — any MCP-capable agent gets typed tool calls, not shell-output parsing.
- `sparks fmt` validates schema deterministically.
- `sparks ingest --prepare` returns structured JSON; agents don't parse markdown separators.
- Agent instructions become 3-5 lines per command.
- Switching agents (or running several) becomes trivial — the protocol is the CLI, not the prose.

## V1 Scope

Sparks v1 is deliberately opinionated. The KB shape is **hardcoded**: `entity / concept / summary / synthesis / collection` page types, a fixed frontmatter schema, a fixed vault layout, a fixed set of collection types. This shape is inspired by Karpathy's LLM-wiki pattern and proven through dogfooding.

V1 assumes:

- **One vault per user** — single-tenant, local filesystem.
- **Raw sources are append-only.** Humans capture in `inbox.md`; existing raw files aren't edited. Corrections flow through new inbox entries and become `## Revision` sections on wiki pages.
- **One person writes; one or more agents maintain.**
- **Git for sync and history.** No server, no database beyond local SQLite.

V1 explicitly does not do: custom page schemas, document ingestion (PDF/DOCX/HTML), multi-user collaboration, mutable raw sources, semantic search, or capture UX beyond `inbox.md`.

## Why Go

- Single static binary — no venv, no runtime dependencies. Drop it on any machine.
- Pure-Go SQLite (`modernc.org/sqlite`) — no CGo, no `brew install sqlite`.
- Natural fit for MCP stdio servers (JSON-RPC over stdin/stdout).
- Fast filesystem walks and SHA-256 hashing — the mechanical layer adds negligible latency.
- Cross-compile for macOS, Linux, Windows from one machine.

## Origin Note

Sparks emerged from dogfooding the Karpathy LLM-wiki pattern against a real personal vault over several weeks. The friction points that surfaced — prose-heavy agent instructions, token burn on mechanical work, fragility when switching agents, Python tooling friction — are what shaped the design. The tool is now standalone; the vault it came from is one instance of what Sparks runs on, not the thing Sparks is.
