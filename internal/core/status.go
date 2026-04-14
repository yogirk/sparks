package core

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// Status returns a high-level summary of the vault: page counts by type,
// inbox pending entries, and manifest stats.
func Status(v *vault.Vault, db *manifest.DB) (StatusReport, error) {
	pages, err := db.CountPagesByType()
	if err != nil {
		return StatusReport{}, err
	}
	total, changed, deleted, err := db.FileCounts()
	if err != nil {
		return StatusReport{}, err
	}
	schemaV, err := db.SchemaVersion()
	if err != nil {
		return StatusReport{}, err
	}
	pending, err := countInboxEntries(v.InboxPath())
	if err != nil {
		return StatusReport{}, err
	}
	return StatusReport{
		VaultRoot:     v.Root,
		VaultName:     v.Config.Vault.Name,
		SchemaVersion: schemaV,
		PagesByType:   pages,
		Files:         FileStats{Total: total, Changed: changed, Deleted: deleted},
		InboxPending:  pending,
	}, nil
}

// countInboxEntries counts entries separated by `---` on its own line.
//
// Convention: the inbox starts with an optional header block. The first
// `---` line marks the boundary between header (instructions, comments)
// and entries. Subsequent `---` lines separate entries from each other.
// Blank entries (whitespace-only chunks between two separators) don't
// count. A missing inbox file returns 0 with no error.
//
// Examples:
//
//	"# header\n---\n"            → 0 (no entries after separator)
//	"# header\n---\nA"           → 1
//	"# header\n---\nA\n---\nB"   → 2
//	"A"                          → 0 (no separator means it's still header)
func countInboxEntries(inboxPath string) (int, error) {
	f, err := os.Open(inboxPath)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	entries := 0
	inHeader := true
	currentHasContent := false

	for scanner.Scan() {
		trimmed := strings.TrimRight(scanner.Text(), "\r")
		if trimmed == "---" {
			if !inHeader && currentHasContent {
				entries++
			}
			inHeader = false
			currentHasContent = false
			continue
		}
		if !inHeader && strings.TrimSpace(trimmed) != "" {
			currentHasContent = true
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if !inHeader && currentHasContent {
		entries++
	}
	return entries, nil
}
