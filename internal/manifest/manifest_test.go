package manifest

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAppliesMigrations(t *testing.T) {
	db := openTestDB(t)
	v, err := db.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != 1 {
		t.Errorf("SchemaVersion = %d, want 1", v)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sparks.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	v, _ := db2.SchemaVersion()
	if v != 1 {
		t.Errorf("schema after reopen = %d, want 1", v)
	}
}

func TestUpsertAndGetFile(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	fr := FileRecord{
		Path:     "wiki/entities/Test.md",
		Hash:     "deadbeef",
		Size:     42,
		MTime:    now,
		ScanTime: now,
	}
	if err := db.UpsertFile(fr); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}
	got, err := db.GetFile("wiki/entities/Test.md")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if got.Hash != "deadbeef" || got.Size != 42 {
		t.Errorf("got %+v", got)
	}
	if !got.MTime.Equal(now) {
		t.Errorf("mtime round-trip mismatch: got %v want %v", got.MTime, now)
	}
}

func TestMarkDeletedAndAllPaths(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()
	for _, p := range []string{"a.md", "b.md", "c.md"} {
		if err := db.UpsertFile(FileRecord{Path: p, Hash: "h", Size: 1, MTime: now, ScanTime: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.MarkDeleted([]string{"b.md"}); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}
	paths, err := db.AllPaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Errorf("AllPaths len = %d, want 2 (b.md should be excluded)", len(paths))
	}
}

func TestUpsertFrontmatterAndCount(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()
	for i, typ := range []string{"entity", "entity", "concept"} {
		path := filepath.Join("wiki", typ+"-"+string(rune('a'+i))+".md")
		if err := db.UpsertFile(FileRecord{Path: path, Hash: "h", Size: 1, MTime: now, ScanTime: now}); err != nil {
			t.Fatal(err)
		}
		if err := db.UpsertFrontmatter(FrontmatterRecord{Path: path, Type: typ, Title: typ}); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := db.CountPagesByType()
	if err != nil {
		t.Fatal(err)
	}
	if counts["entity"] != 2 || counts["concept"] != 1 {
		t.Errorf("counts = %v, want entity=2 concept=1", counts)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "sparks.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
