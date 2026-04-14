package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

func TestInitVaultFresh(t *testing.T) {
	dir := t.TempDir()
	res, err := InitVault(dir)
	if err != nil {
		t.Fatalf("InitVault: %v", err)
	}
	if !res.Created || res.Existed {
		t.Errorf("flags = %+v, want Created=true Existed=false", res)
	}
	if !res.ManifestNew {
		t.Error("ManifestNew = false on fresh init")
	}
	if _, err := os.Stat(filepath.Join(dir, "sparks.db")); err != nil {
		t.Errorf("sparks.db missing: %v", err)
	}
}

func TestInitVaultExistingReports(t *testing.T) {
	dir := t.TempDir()
	if _, err := InitVault(dir); err != nil {
		t.Fatal(err)
	}
	res, err := InitVault(dir)
	if err != nil {
		t.Fatalf("second InitVault: %v", err)
	}
	if !res.Existed || res.Created {
		t.Errorf("flags = %+v, want Existed=true Created=false", res)
	}
}

func TestScanIncrementalSkipsUnchanged(t *testing.T) {
	v, db := freshVault(t)

	// First scan: everything hashed.
	r1, err := Scan(v, db)
	if err != nil {
		t.Fatalf("scan 1: %v", err)
	}
	if r1.Hashed == 0 {
		t.Fatal("first scan hashed nothing")
	}

	// Second scan immediately after: nothing changed, everything skipped.
	r2, err := Scan(v, db)
	if err != nil {
		t.Fatalf("scan 2: %v", err)
	}
	if r2.Hashed != 0 {
		t.Errorf("scan 2 Hashed = %d, want 0", r2.Hashed)
	}
	if r2.Skipped != r1.Walked {
		t.Errorf("scan 2 Skipped = %d, want %d", r2.Skipped, r1.Walked)
	}
}

func TestScanDetectsDeletedFiles(t *testing.T) {
	v, db := freshVault(t)
	probe := filepath.Join(v.Root, "wiki", "entities", "Probe.md")
	writeFile(t, probe, "---\ntitle: Probe\ntype: entity\nmaturity: seed\nsources: [raw/inbox/2026.md]\ncreated: 2026-04-14\nupdated: 2026-04-14\n---\n\nbody\n")

	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	r, err := Scan(v, db)
	if err != nil {
		t.Fatal(err)
	}
	if r.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", r.Deleted)
	}
}

func TestScanPopulatesFrontmatterForWikiPages(t *testing.T) {
	v, db := freshVault(t)
	page := filepath.Join(v.Root, "wiki", "entities", "Cascade.md")
	writeFile(t, page, `---
title: Cascade
type: entity
maturity: working
tags: [gcp]
sources: [raw/inbox/2026-04-01.md]
created: 2026-04-01
updated: 2026-04-13
---

Body.
`)
	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}
	counts, err := db.CountPagesByType()
	if err != nil {
		t.Fatal(err)
	}
	if counts["entity"] != 1 {
		t.Errorf("counts = %v, want entity=1", counts)
	}
}

func TestStatusEmptyVaultZeroPending(t *testing.T) {
	v, db := freshVault(t)
	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}
	rep, err := Status(v, db)
	if err != nil {
		t.Fatal(err)
	}
	if rep.InboxPending != 0 {
		t.Errorf("fresh vault InboxPending = %d, want 0 (seed inbox is header-only)", rep.InboxPending)
	}
	if rep.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", rep.SchemaVersion)
	}
}

func TestStatusCountsInboxEntries(t *testing.T) {
	v, db := freshVault(t)

	// Append two real entries below the seed `---` separator.
	f, err := os.OpenFile(v.InboxPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("First capture\n---\nSecond capture\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}
	rep, err := Status(v, db)
	if err != nil {
		t.Fatal(err)
	}
	if rep.InboxPending != 2 {
		t.Errorf("InboxPending = %d, want 2", rep.InboxPending)
	}
}

// freshVault creates a brand-new vault in a temp dir and returns the open
// vault + manifest. Cleanup is registered with t.
func freshVault(t *testing.T) (*vault.Vault, *manifest.DB) {
	t.Helper()
	dir := t.TempDir()
	if _, err := InitVault(dir); err != nil {
		t.Fatalf("InitVault: %v", err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	db, err := manifest.Open(v.ManifestPath())
	if err != nil {
		t.Fatalf("manifest.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return v, db
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
