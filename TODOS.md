# Sparks — TODOS

Post-v1 work tracked here. v1 scope is frozen in `~/.gstack/projects/sparks/rk-unknown-design-20260414-180325.md`. This file is the living backlog.

---

## Perf targets (v1.1)

**What.** Measure binary size and runtime. Land a CI check that fails if:
- binary > 20 MB
- `sparks status` > 50ms on a 1000-file vault

**Why.** Size and latency are a real part of "Sparks feels fast." We deferred measurement out of v1 to ship, but if we let them drift we'll regret it.

**Pros.** Keeps Sparks feeling instant. Catches regressions.
**Cons.** CI perf gates are flaky on shared runners. Need reliable fixture vault.

**Context.** v1 ships with no perf gate. First time a user says "sparks scan is slow," we'll know we skipped this and regret it. The fixture vault for the bench lives at `testdata/bench/vault/` (needs populating).

**Depends on.** v1 released. Benchmark fixture vault of ~1000 files representative of real use.

---

## HTTP MCP sidecar (v1.1 or v2)

**What.** Optional `sparks serve --http :port` that exposes MCP over Streamable HTTP instead of stdio.

**Why.** Some harnesses (IDE integrations, web-based agents) can't spawn stdio subprocesses. HTTP transport unlocks them.

**Pros.** Broader harness compatibility. Enables remote agents driving a vault over SSH tunnels or similar.
**Cons.** Introduces a listening socket (security surface). Needs auth/origin checks to be safe. Spec for MCP Streamable HTTP is newer and moving.

**Context.** Stdio is enough for v1 because Claude Code, Codex CLI, and Gemini CLI all spawn stdio subprocesses. If a stranger asks for HTTP, that's the demand signal.

**Depends on.** v1 MCP working and stable. MCP Streamable HTTP spec matured and `mcp-go` supports it.

---

## `sparks watch` (fsnotify, v2)

**What.** Long-running daemon mode that watches the vault for changes and incrementally updates the manifest in real time.

**Why.** Removes the `sparks scan` step before every ingest. Agents see fresh manifest without round-trips.

**Pros.** Lower latency for agent ops. Could power live `sparks lint` feedback during authoring.
**Cons.** Long-running process is more failure surface. Watch descriptors leak on bad filesystems. Cross-platform fsnotify is fiddly.

**Context.** Deferred because v1's on-demand `scan` is fast enough (incremental, mtime-keyed). `watch` only pays off at high vault-churn, which isn't the v1 use case.

**Depends on.** v1 stable incremental scan. Real user demand.

---

## Declarative KB shape (v2, demand-driven)

**What.** Let users define their own page types, frontmatter schema, and collection extractors via a config file instead of hardcoding the Karpathy 5-type shape.

**Why.** v1 is opinionated. If strangers want to use Sparks for a KB that isn't Karpathy-shaped (daily notes, meeting notes, literature notes, etc.), they need to hack the binary.

**Pros.** Broadens the addressable KB styles.
**Cons.** Explodes testing surface. Config-driven extractors are a classic abstraction trap — starts simple, becomes its own DSL. Every shape quirk becomes a user-facing feature to document.

**Context.** Explicit v2 trajectory from the office-hours session. Ship the opinion first, earn the right to generalize. Do NOT do this pre-emptively. Wait for 3+ external users asking for a specific shape change.

**Depends on.** v1 shipped, real external adoption, repeated identical shape requests.

---

## Docs site (v1.1)

**What.** Proper documentation site beyond the README + `sparks describe` output. Cover installation, agent setup per harness, dogfood walkthrough, troubleshooting.

**Why.** Strangers installing Sparks benefit from stepwise onboarding. README is a landing page, not a manual.

**Pros.** Reduces "how do I get started" friction. Surfaces the `any harness, any model` story.
**Cons.** Docs site infrastructure is its own yak (Docusaurus? MkDocs? mdbook? yogirk.dev subdirectory?). Maintenance burden.

**Context.** v1 ships with README + `sparks describe`. If that's not enough, users will say so.

**Depends on.** Real user friction reports.

---

## Structured logging / verbose mode (v1.0, spec out before scaffold)

**What.** Decide and implement a consistent `-v`/`--verbose`/`--json`/`--quiet` story across all commands.

**Why.** Operational debugging requires it. Without a plan upfront, every command gets its own ad-hoc log format.

**Pros.** Makes bug reports possible.
**Cons.** One more thing to design before Week 1 scaffold.

**Context.** Captured here so it isn't forgotten. Likely lives as a small `internal/log/` package wrapping `slog` (Go 1.21+). Command flags normalized via a shared `persistentFlag` in cobra root.

**Depends on.** Week 1 scaffold (should be addressed before the first command ships).

---

## Decouple capture from ingest (post-v2, demand-driven)

**What.** Let `sparks ingest --prepare` accept sources other than `inbox.md`. A `--source` flag taking a file path, stdin, or (later) a URL — so capture isn't wedded to one append-only markdown file at the vault root.

**Why.** `inbox.md` is one capture cadence among many. Someone running Sparks on meeting notes, a Slack export, a `~/Dropbox/captures/` folder, or piped clippings from a browser extension shouldn't have to pre-concatenate everything into `inbox.md` first. The ingest protocol (prepare → agent writes pages → finalize) is already source-agnostic under the hood; the CLI just hardcodes the source today.

**Pros.** Generalizes Sparks beyond the maintainer's capture habit without touching the core shape. Opens the door to non-text sources later (RSS, email, clippings) by keeping the source interface a thin seam.
**Cons.** Multiplies "what does finalize archive to?" edge cases — today we move `inbox.md` to `raw/inbox/YYYY-MM-DD.md` atomically. With arbitrary sources, archive semantics need to be per-source-type (copy? move? nothing?). Another config surface.

**Context.** Raised during the pre-release positioning review (2026-04-22). The positioning is already harness-agnostic; this would make it *capture-agnostic* too. Not v1 and not v2 — tracked here so we notice if strangers start asking for it. The right trigger is a concrete second capture mode from a real user, not speculation.

**Depends on.** v1 shipped, real external adoption, and at least one user describing a capture flow that `inbox.md` doesn't fit.

---

## Multi-user coordination (v2+, probably never)

**What.** True multi-writer support (multiple humans editing the same vault simultaneously).

**Why.** Opens Sparks beyond personal-scale.

**Pros.** Broader use cases.
**Cons.** Requires rethinking append-only raw contract, revision model, manifest ownership. Totally different product.

**Context.** Explicitly out of v1 scope. Noting here only to capture that it exists as a theoretical future — not a planned trajectory.

**Depends on.** A reason to care beyond personal KBs. Don't do this on speculation.
