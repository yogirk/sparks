// Package core holds the business logic that both CLI commands and MCP
// handlers wrap. Every function returns typed structs; the caller decides
// whether to format them as text, JSON, or an MCP tool result.
//
// This package must NOT import cobra, mcp-go, or any presentation-layer
// dependency. It's the keystone of decision A1 (thin adapters): if you
// find yourself adding flags or printing here, you're in the wrong layer.
package core

import "time"

// InitResult describes what `sparks init` did.
type InitResult struct {
	VaultRoot   string `json:"vault_root"`
	Created     bool   `json:"created"`
	Existed     bool   `json:"existed"`
	ManifestNew bool   `json:"manifest_new"`
}

// ScanResult is the summary of a manifest scan.
type ScanResult struct {
	VaultRoot string        `json:"vault_root"`
	Walked    int           `json:"walked"`
	Hashed    int           `json:"hashed"`  // files actually rehashed (mtime/size changed)
	Skipped   int           `json:"skipped"` // unchanged, hash skipped
	Deleted   int           `json:"deleted"` // newly marked deleted this scan
	Errors    []ScanError   `json:"errors,omitempty"`
	Duration  time.Duration `json:"duration_ns"`
}

// ScanError records a per-file failure so a partial scan can still finish.
type ScanError struct {
	Path string `json:"path"`
	Err  string `json:"err"`
}

// StatusReport is the high-level vault overview shown by `sparks status`.
type StatusReport struct {
	VaultRoot     string         `json:"vault_root"`
	VaultName     string         `json:"vault_name"`
	SchemaVersion int            `json:"schema_version"`
	PagesByType   map[string]int `json:"pages_by_type"`
	Files         FileStats      `json:"files"`
	InboxPending  int            `json:"inbox_pending"`
}

// FileStats summarizes the manifest's file table.
type FileStats struct {
	Total   int `json:"total"`
	Changed int `json:"changed"` // not yet ingested
	Deleted int `json:"deleted"`
}

// BriefReport is the structured plumbing snapshot feeding `sparks brief`.
// Synthesis is agent work: the CLI gathers signals, the agent reads the
// JSON and writes the weekly brief.
type BriefReport struct {
	VaultRoot   string        `json:"vault_root"`
	Window      BriefWindow   `json:"window"`
	LogEntries  []BriefLogDay `json:"log_entries"`
	NewRaw      []BriefFile   `json:"new_raw"`
	UpdatedWiki []BriefPage   `json:"updated_wiki"`
	Revisit     BriefRevisit  `json:"revisit"`
	Tasks       BriefTasks    `json:"tasks"`
}

// BriefWindow records the time range the report covers. Since is
// inclusive, Now is the wall-clock moment the report was built.
type BriefWindow struct {
	Days  int       `json:"days"`
	Since time.Time `json:"since"`
	Now   time.Time `json:"now"`
}

// BriefLogDay is one `## [YYYY-MM-DD] …` heading from wiki/log.md with
// the body that follows it, up to the next heading or EOF.
type BriefLogDay struct {
	Date string `json:"date"` // YYYY-MM-DD
	Text string `json:"text"` // original heading + body block
}

// BriefFile is a raw capture file surfaced as "new in the window."
type BriefFile struct {
	Path  string    `json:"path"`
	MTime time.Time `json:"mtime"`
}

// BriefPage is a wiki page whose frontmatter `updated:` falls in the window.
type BriefPage struct {
	Path     string `json:"path"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Maturity string `json:"maturity"`
	Updated  string `json:"updated"` // YYYY-MM-DD
}

// BriefRevisit groups deterministic "worth revisiting" signals. These
// reuse the same checks `sparks lint` runs; brief just projects them.
type BriefRevisit struct {
	Stale       []string `json:"stale"`        // wiki older than source mtime
	SeedOrphans []string `json:"seed_orphans"` // maturity=seed with no incoming links
	Thin        []string `json:"thin"`         // fewer than 3 sentences of body
}

// BriefTasks summarizes open tasks from wiki/collections/Tasks.md.
type BriefTasks struct {
	Open  int      `json:"open"`
	First []string `json:"first,omitempty"` // preview of the first few
}
