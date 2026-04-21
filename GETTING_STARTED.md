# Getting Started with Sparks

You have an AI coding harness — Claude Code, Codex CLI, Gemini CLI, or something MCP-capable. You want it to maintain a personal knowledge base for you. This guide gets you from zero to working in about ten minutes.

The model does the thinking. Sparks does the plumbing. You just capture.

---

## 1. Install Sparks

Pick one:

```bash
# Homebrew (macOS / Linux)
brew install yogirk/tgcp/sparks

# Or via Go (any platform with a Go toolchain)
go install github.com/yogirk/sparks/cmd/sparks@latest

# Or grab a binary from https://github.com/yogirk/sparks/releases
```

Verify:

```bash
sparks --version
sparks describe | head      # prints the contract embedded in the binary
```

If `sparks describe` works, the install is good — that's the runtime telling you what it expects from any agent that drives it.

---

## 2. Create a vault

A vault is just a directory with a `sparks.toml` marker. Pick a path you'll back up (typically inside `~/Projects/notes/` or similar).

```bash
mkdir -p ~/Projects/notes/my-vault
cd ~/Projects/notes/my-vault
```

Now wire it up for your harness. Pick the right command:

| Harness | Command | What it writes |
|---|---|---|
| Claude Code | `sparks init --agent claude` | `CLAUDE.md` |
| Codex CLI | `sparks init --agent codex` | `AGENTS.md` |
| Gemini CLI | `sparks init --agent gemini` | `GEMINI.md` |
| Anything else | `sparks init --agent generic` | `AGENTS.md` |

Example:

```bash
sparks init --agent claude
```

This single command creates everything:

- `sparks.toml` — vault config
- `sparks.db` — the manifest (SQLite, hidden from your editor)
- `inbox.md` — where you capture
- `raw/` and `wiki/` directory layout
- `CLAUDE.md` (or whatever filename matches your harness) — the embedded contract telling the agent how to operate the vault

The instruction file is identical content across all harnesses. Only the filename differs to match each tool's convention. You can run `--agent X` multiple times to get multiple instruction files in the same vault if you switch harnesses.

**Optional but recommended:** turn the vault into a git repo so Sparks can auto-commit on every ingest:

```bash
git init -b main
git add . && git commit -m "init vault"
```

If you'd rather Sparks not auto-commit, edit `sparks.toml` and set `auto_commit = false` under `[git]`.

---

## 3. Launch your harness in the vault

`cd` into the vault directory and start your AI tool there. The instruction file (CLAUDE.md / AGENTS.md / GEMINI.md) is at the vault root, so the harness picks it up automatically when launched in that directory.

```bash
cd ~/Projects/notes/my-vault

# Then one of:
claude        # Claude Code reads CLAUDE.md
codex         # Codex CLI reads AGENTS.md
gemini        # Gemini CLI reads GEMINI.md
```

That's it. Your agent now knows the Sparks contract. No prose to write, no system prompt to maintain.

---

## 4. The daily flow

### Capture

You write into `inbox.md`. Free-form. Whenever you have a thought.

```markdown
# Inbox

Drop captures below the line, separated by `---` on its own line.

---

2026-04-15
Had a thought about how runtime contracts compare to API specs.
The difference: a contract describes the protocol, not the calls.

---

book: The Soul of a New Machine / Tracy Kidder
to-read: https://paulgraham.com/superlinear.html
- [ ] Write the v0.2 changelog
```

The header (above the first `---`) is for instructions to yourself. Everything below `---` is entries, separated by more `---` lines. Each entry can start with a `YYYY-MM-DD` line if you want to backdate it; otherwise today is used.

### Ingest

When you're ready (end of day, end of week, whenever), tell your agent:

> "ingest my inbox"

The agent will:

1. Run `sparks ingest --prepare` — gets a structured JSON of your entries with deterministic hints (URLs detected, tasks detected, etc.)
2. Read each entry, decide what wiki pages to create or update, write them in `wiki/entities/`, `wiki/concepts/`, `wiki/synthesis/`, etc.
3. Run `sparks ingest --finalize` — archives the inbox to `raw/inbox/YYYY-MM-DD.md`, clears the inbox header-preserving, rescans, commits via your git identity if `auto_commit = true`

You see the result in your wiki: new pages, updated pages, growing collections.

### Query

Ask the agent things like:

> "what do my notes say about agent runtimes?"
> "what books have I been quoting from?"
> "what's the link graph around Cascade?"

The agent uses `sparks query` for structured lookups (by title, tag, link graph) and reads the matching wiki pages directly. Synthesis is the agent's job, not the CLI's — Sparks doesn't do semantic search.

### Tasks

Drop tasks in the inbox like `- [ ] thing to do`. After ingest, they land in `wiki/collections/Tasks.md` under the right project heading.

Mark them done from the command line:

```bash
sparks done "thing to do"   # fuzzy-matches; ambiguous matches list candidates
```

Or ask the agent: "mark X as done."

### Weekly brief

Ask the agent: "give me a brief of this past week." Under the hood it
runs `sparks brief --json` to pull the recent log entries, new raw
captures, updated wiki pages, things worth revisiting, and open tasks,
then writes the synthesis. Synthesis is the agent's job; Sparks only
gathers the signals.

```bash
sparks brief              # human summary
sparks brief --json       # structured snapshot for agent synthesis
sparks brief --days 30    # custom window
```

### Health checks

Once a week or so, run:

```bash
sparks lint            # orphans, broken links, thin pages, duplicate aliases
sparks status          # vault overview
sparks collections regen  # rebuild Quotes, Bookmarks, Books, etc.
sparks index           # rebuild wiki/index.md preserving your descriptions
```

Lint will surface real data hygiene issues — pages nobody links to, frontmatter typos, source files that disappeared. Most are small fixes; ask your agent to clean them up.

### Browse your vault

Don't want to install Obsidian? Sparks has a built-in viewer:

```bash
sparks view --open     # opens http://127.0.0.1:3030
```

Three-column layout: navigation on the left, your wiki page in the center (serif typography, narrow measure), metadata + backlinks + tags on the right. Wikilinks are live — click through your knowledge graph the same way an agent traverses it.

Read-only. Edits happen in your editor or via the agent. Changes show up on next page refresh.

---

## 5. MCP integration (optional, advanced)

If your harness speaks MCP (Claude Code does), you can register Sparks as an MCP server instead of having the agent shell out to the CLI. The 11 operations become native tools (`sparks_status`, `sparks_lint`, `sparks_prepare_ingest`, etc.).

For Claude Code, add to your `~/.config/claude-code/mcp.json` (or equivalent):

```json
{
  "mcpServers": {
    "sparks": {
      "command": "sparks",
      "args": ["serve"],
      "cwd": "/Users/you/Projects/notes/my-vault"
    }
  }
}
```

The `cwd` field tells the server which vault to operate on. If you have multiple vaults, register multiple MCP servers with different `cwd` values.

The CLI and MCP surfaces share the same internal core — you can use both at the same time without conflict (Sparks uses SQLite WAL mode and a concurrent-ingest lock).

---

## 6. Multi-agent vault

Same vault, multiple harnesses? No problem. Run `sparks init --agent X` once per harness:

```bash
cd ~/Projects/notes/my-vault
sparks init --agent claude   # CLAUDE.md
sparks init --agent codex    # AGENTS.md
sparks init --agent gemini   # GEMINI.md
```

Now you can launch any of them in that directory and they all read the same contract. The vault doesn't care which one is driving — it cares that whoever drives respects the contract.

If two are running concurrently and both try `sparks ingest --prepare`, the second one gets a clear "already in_progress" error. Use `sparks ingest --abort` to recover.

---

## 7. Troubleshooting

**`sparks: vault not found`**
You're outside the vault directory, or `sparks.toml` is missing. `cd` into the vault, or run `sparks init` to create one.

**`already in_progress` on ingest**
A prior `--prepare` didn't finish. `sparks ingest --abort` clears the lock.

**Agent doesn't seem to know the contract**
Restart the agent. Most harnesses load instruction files at startup, not on every prompt. Or explicitly tell it: "read CLAUDE.md first."

**Lots of broken links after import**
You imported a vault that uses different conventions. The resolver matches by title, alias, then filename. If your wikilinks are to article titles (e.g., bookmarks), regenerate the affected collections — Sparks's format uses markdown links, not wikilinks.

**Lots of stale-pages after `cp -R`**
Use `cp -Rp` to preserve mtimes. Or accept the noise — `sparks scan` will catch up over time as you actually edit pages.

**Releases / GitHub Actions not building**
Check the release workflow logs. If brew fails specifically, you need `HOMEBREW_TAP_GITHUB_TOKEN` set as a repo secret pointing to a PAT with write access to your tap repo.

---

## 8. What to read next

- [`README.md`](README.md) — install + commands at a glance
- [`sparks-contracts.md`](sparks-contracts.md) — full agent-runtime contract (also embedded; `sparks describe`)
- [`sparks-spec.md`](sparks-spec.md) — full v1 specification
- [`sparks-insight.md`](sparks-insight.md) — the one-page thesis behind why this exists
- [`CHANGELOG.md`](CHANGELOG.md) — what shipped in each version
- [`TODOS.md`](TODOS.md) — post-v1 work tracked here

Found a rough edge? [Open an issue](https://github.com/yogirk/sparks/issues). Pre-alpha means bugs are normal and reports are gold.
