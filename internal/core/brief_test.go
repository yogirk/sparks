package core

import (
	"path/filepath"
	"testing"
	"time"
)

// TestBriefEmptyVault exercises the happy path on a freshly initialized
// vault: nothing new, no log, no tasks. Every slice should be empty and
// the window should reflect the caller's request.
func TestBriefEmptyVault(t *testing.T) {
	v, db := freshVault(t)
	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}

	rep, err := Brief(v, db, 7)
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if rep.Window.Days != 7 {
		t.Errorf("Window.Days = %d, want 7", rep.Window.Days)
	}
	if len(rep.LogEntries) != 0 {
		t.Errorf("LogEntries = %d, want 0", len(rep.LogEntries))
	}
	if len(rep.NewRaw) != 0 {
		t.Errorf("NewRaw = %d, want 0", len(rep.NewRaw))
	}
	if rep.Tasks.Open != 0 {
		t.Errorf("Tasks.Open = %d, want 0", rep.Tasks.Open)
	}
}

// TestBriefDaysZeroDefaults covers the contract that a 0 or negative
// days argument falls back to DefaultBriefDays. Agents can pass 0 to
// mean "you pick."
func TestBriefDaysZeroDefaults(t *testing.T) {
	v, db := freshVault(t)
	rep, err := Brief(v, db, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Window.Days != DefaultBriefDays {
		t.Errorf("Window.Days = %d, want %d", rep.Window.Days, DefaultBriefDays)
	}
}

// TestBriefLogWindow verifies parseLogWindow chunks by `## [YYYY-MM-DD]`
// headings, filters by window, and orders most-recent-first. Uses dates
// relative to today so the test stays green across days.
func TestBriefLogWindow(t *testing.T) {
	v, db := freshVault(t)

	today := time.Now()
	recent := today.AddDate(0, 0, -1).Format("2006-01-02")
	old := today.AddDate(0, 0, -30).Format("2006-01-02")

	logBody := "# Log\n\n" +
		"## [" + recent + "] ingest | recent change\n\nrecent body\n\n" +
		"## [" + old + "] ingest | old change\n\nold body\n"
	writeFile(t, filepath.Join(v.Root, "wiki", "log.md"), logBody)

	rep, err := Brief(v, db, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.LogEntries) != 1 {
		t.Fatalf("LogEntries = %d, want 1 (old entry should be filtered out)", len(rep.LogEntries))
	}
	if rep.LogEntries[0].Date != recent {
		t.Errorf("LogEntries[0].Date = %q, want %q", rep.LogEntries[0].Date, recent)
	}
	if !containsSubstring(rep.LogEntries[0].Text, "recent body") {
		t.Errorf("log body not captured: %q", rep.LogEntries[0].Text)
	}
}

// TestBriefNewRawExcludesAutoDirs confirms raw/inbox/ (CLI-written
// archive) and raw/archive/ (human retirement) don't show up as "new
// raw" signal.
func TestBriefNewRawExcludesAutoDirs(t *testing.T) {
	v, db := freshVault(t)

	writeFile(t, filepath.Join(v.Root, "raw", "ideas", "fresh.md"), "fresh idea\n")
	writeFile(t, filepath.Join(v.Root, "raw", "inbox", "2026-04-14.md"), "cli-written\n")
	writeFile(t, filepath.Join(v.Root, "raw", "archive", "retired.md"), "archived\n")

	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}
	rep, err := Brief(v, db, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.NewRaw) != 1 {
		t.Fatalf("NewRaw paths = %+v, want only raw/ideas/fresh.md", rep.NewRaw)
	}
	if rep.NewRaw[0].Path != "raw/ideas/fresh.md" {
		t.Errorf("NewRaw[0].Path = %q, want raw/ideas/fresh.md", rep.NewRaw[0].Path)
	}
}

// TestBriefUpdatedWikiFilter checks that pages with updated: inside the
// window are surfaced and older ones dropped, ordered most-recent first.
func TestBriefUpdatedWikiFilter(t *testing.T) {
	v, db := freshVault(t)

	today := time.Now().Format("2006-01-02")
	long := time.Now().AddDate(0, 0, -40).Format("2006-01-02")

	writeFile(t, filepath.Join(v.Root, "wiki", "entities", "Recent.md"),
		"---\ntitle: Recent\ntype: entity\nmaturity: seed\nsources: [raw/ideas/x.md]\ncreated: "+today+"\nupdated: "+today+"\n---\n\nbody.\n")
	writeFile(t, filepath.Join(v.Root, "wiki", "entities", "Ancient.md"),
		"---\ntitle: Ancient\ntype: entity\nmaturity: working\nsources: [raw/ideas/y.md]\ncreated: "+long+"\nupdated: "+long+"\n---\n\nbody.\n")
	writeFile(t, filepath.Join(v.Root, "raw", "ideas", "x.md"), "x\n")
	writeFile(t, filepath.Join(v.Root, "raw", "ideas", "y.md"), "y\n")

	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}
	rep, err := Brief(v, db, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.UpdatedWiki) != 1 {
		t.Fatalf("UpdatedWiki = %+v, want 1 match", rep.UpdatedWiki)
	}
	if rep.UpdatedWiki[0].Title != "Recent" {
		t.Errorf("UpdatedWiki[0].Title = %q, want Recent", rep.UpdatedWiki[0].Title)
	}
}

// TestBriefTasksCountAndPreview makes sure open tasks are counted and a
// preview is returned, and that completed tasks are excluded.
func TestBriefTasksCountAndPreview(t *testing.T) {
	v, db := freshVault(t)

	tasks := "# Tasks\n\n## [[Sparks]]\n\n- [ ] first open\n- [ ] second open\n- [x] done already\n"
	writeFile(t, filepath.Join(v.Root, TasksFilename), tasks)
	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}

	rep, err := Brief(v, db, 7)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Tasks.Open != 2 {
		t.Errorf("Tasks.Open = %d, want 2", rep.Tasks.Open)
	}
	if len(rep.Tasks.First) != 2 {
		t.Errorf("Tasks.First = %v, want 2 entries", rep.Tasks.First)
	}
	if len(rep.Tasks.First) >= 1 && rep.Tasks.First[0] != "first open" {
		t.Errorf("Tasks.First[0] = %q, want \"first open\"", rep.Tasks.First[0])
	}
}

// TestBriefRevisitSeedOrphans exercises the seed-orphan detection path
// in briefRevisit. A seed page with no incoming links (other than index)
// must appear; a seed page that's linked from another page must NOT.
// Stale/thin signals are owned by lint and covered there — brief just
// projects them — so this test focuses on the brief-specific logic.
func TestBriefRevisitSeedOrphans(t *testing.T) {
	v, db := freshVault(t)

	today := time.Now().Format("2006-01-02")

	// Orphan seed: no incoming links from anything except possibly index.
	writeFile(t, filepath.Join(v.Root, "wiki", "entities", "Orphan.md"),
		"---\ntitle: Orphan\ntype: entity\nmaturity: seed\nsources: [raw/ideas/o.md]\ncreated: "+today+"\nupdated: "+today+"\n---\n\nLonely seed.\n")
	// Linked seed: another page wikilinks to it, so it's NOT an orphan.
	writeFile(t, filepath.Join(v.Root, "wiki", "entities", "Linked.md"),
		"---\ntitle: Linked\ntype: entity\nmaturity: seed\nsources: [raw/ideas/l.md]\ncreated: "+today+"\nupdated: "+today+"\n---\n\nReferenced seed.\n")
	writeFile(t, filepath.Join(v.Root, "wiki", "entities", "Linker.md"),
		"---\ntitle: Linker\ntype: entity\nmaturity: working\nsources: [raw/ideas/lk.md]\ncreated: "+today+"\nupdated: "+today+"\n---\n\nSee [[Linked]] for context.\n")
	writeFile(t, filepath.Join(v.Root, "raw", "ideas", "o.md"), "o\n")
	writeFile(t, filepath.Join(v.Root, "raw", "ideas", "l.md"), "l\n")
	writeFile(t, filepath.Join(v.Root, "raw", "ideas", "lk.md"), "lk\n")

	if _, err := Scan(v, db); err != nil {
		t.Fatal(err)
	}
	rep, err := Brief(v, db, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Revisit.SeedOrphans) != 1 {
		t.Fatalf("SeedOrphans = %+v, want exactly [wiki/entities/Orphan.md]", rep.Revisit.SeedOrphans)
	}
	if rep.Revisit.SeedOrphans[0] != "wiki/entities/Orphan.md" {
		t.Errorf("SeedOrphans[0] = %q, want wiki/entities/Orphan.md", rep.Revisit.SeedOrphans[0])
	}
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
