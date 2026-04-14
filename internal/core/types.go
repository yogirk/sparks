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
