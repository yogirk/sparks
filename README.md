# Sparks

**Knowledge base runtime for AI agents.** A single Go binary that maintains the mechanical integrity of a personal-scale knowledge base so any AI agent — Claude Code, Codex CLI, Gemini CLI, or whatever ships next — can operate it through one CLI.

Not a notes app. Not an editor. Not a sync tool. A runtime.

## Why

Personal knowledge management tools are designed for human interaction. But the emerging pattern is different: humans capture, AI agents maintain. When the operational logic lives as prose in a single agent's config file (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`), the agent ends up doing two very different kinds of work.

1. **Mechanical** — parsing frontmatter, splitting inbox entries, hashing files, detecting orphan links, regenerating collection indexes, archiving, git commits. Deterministic. Doesn't need a language model.
2. **Semantic** — understanding content, deciding what pages to create, writing page content, synthesizing answers, spotting contradictions. The work a model is actually good at.

Trapping mechanical work in prose couples the vault to one agent, burns tokens on regex, and makes the whole system brittle. Sparks extracts the mechanical layer into a binary. Agents do content work. Sparks does plumbing.

Agent instructions collapse from ~200 lines to:

```
1. sparks ingest --prepare
2. Read its output. Create or update wiki pages for each entry.
3. sparks ingest --finalize
```

## Status

Under active construction. v0.1.0 ships around mid-2026. See [`sparks-insight.md`](sparks-insight.md), [`sparks-spec.md`](sparks-spec.md), and [`sparks-contracts.md`](sparks-contracts.md) for the design.

## Install

Not yet released. Build from source:

```bash
git clone https://github.com/yogirk/sparks
cd sparks
go build ./cmd/sparks
```

Once v0.1.0 ships:

```bash
# Homebrew (macOS/Linux)
brew install yogirk/tap/sparks

# Go install
go install github.com/yogirk/sparks/cmd/sparks@latest

# GitHub Releases: prebuilt binaries for darwin/linux/windows on amd64 and arm64
```

## Quick start

```bash
mkdir ~/Projects/notes/my-vault
cd ~/Projects/notes/my-vault
sparks init                    # create sparks.toml, sparks.db, dir layout
sparks init --agent claude     # drop a CLAUDE.md so Claude Code knows the protocol
# ...then let your agent drive
```

Run `sparks describe` to print the canonical agent contract. Run `sparks --help` for the full command surface.

## Architecture

Three layers:

```
┌─────────────────────────────────────────────┐
│ raw/       ← append-only human capture      │
├─────────────────────────────────────────────┤
│ wiki/      ← agent-maintained derived view  │
├─────────────────────────────────────────────┤
│ sparks.db  ← CLI-maintained manifest        │
└─────────────────────────────────────────────┘
```

Raw is the source of truth. Wiki is a derived view. Manifest tracks state for incremental operations. All three are plain files on disk, versioned by git.

## Philosophy

The harness should be thin. Tools should be specialized. Let models reason and process language. Let binaries handle file hashing, frontmatter parsing, and link graph traversal. When the runtime encodes the contract, a new agent can drive the vault without anyone rewriting prose.

## License

TBD before v0.1.0. Probably MIT or Apache 2.0.
