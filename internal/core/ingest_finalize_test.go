package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalizeRequiresPriorPrepare(t *testing.T) {
	v, db := freshVault(t)
	_, err := FinalizeIngest(v, db, "")
	if err == nil || !strings.Contains(err.Error(), "no in_progress ingest") {
		t.Errorf("err = %v, want 'no in_progress ingest'", err)
	}
}

func TestFinalizeArchivesAndClearsInbox(t *testing.T) {
	v, db := freshVault(t)
	writeInbox(t, v.InboxPath(), `# Inbox
---
2026-04-10
First capture.
---
Second capture.
`)

	if _, err := PrepareIngest(v, db); err != nil {
		t.Fatal(err)
	}
	res, err := FinalizeIngest(v, db, "ingest: test")
	if err != nil {
		t.Fatalf("FinalizeIngest: %v", err)
	}
	if res.EntryCount != 2 {
		t.Errorf("EntryCount = %d, want 2", res.EntryCount)
	}

	// Archived file exists and contains both captures.
	archive := readVaultFile(t, v, "raw/inbox/2026-04-10.md")
	if !strings.Contains(archive, "First capture") {
		t.Errorf("archive missing first entry:\n%s", archive)
	}
	// Inbox is cleared of entries but retains the header.
	inboxAfter := readVaultFile(t, v, "inbox.md")
	if strings.Contains(inboxAfter, "First capture") {
		t.Errorf("inbox still has entries after finalize:\n%s", inboxAfter)
	}
	if !strings.Contains(inboxAfter, "---") {
		t.Errorf("inbox header separator stripped:\n%s", inboxAfter)
	}
}

func TestFinalizeSkipsCommitWhenNotRepo(t *testing.T) {
	v, db := freshVault(t)
	writeInbox(t, v.InboxPath(), "# Inbox\n---\nentry\n")
	if _, err := PrepareIngest(v, db); err != nil {
		t.Fatal(err)
	}
	res, err := FinalizeIngest(v, db, "")
	if err != nil {
		t.Fatalf("FinalizeIngest: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (non-repo)", res.CommitSHA)
	}
	if !strings.Contains(res.CommitSkipped, "git repository") {
		t.Errorf("CommitSkipped = %q, want mention of 'git repository'", res.CommitSkipped)
	}
}

func TestFinalizeCommitsInGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	v, db := freshVault(t)
	initGitRepo(t, v.Root)

	writeInbox(t, v.InboxPath(), "# Inbox\n---\nentry one\n---\nentry two\n")
	if _, err := PrepareIngest(v, db); err != nil {
		t.Fatal(err)
	}
	res, err := FinalizeIngest(v, db, "ingest: e2e")
	if err != nil {
		t.Fatalf("FinalizeIngest: %v", err)
	}
	if len(res.CommitSHA) < 7 {
		t.Errorf("CommitSHA = %q, want short hash", res.CommitSHA)
	}

	// Verify the commit actually landed.
	cmd := exec.Command("git", "-C", v.Root, "log", "--oneline", "-1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "ingest: e2e") {
		t.Errorf("last commit = %q, want contains 'ingest: e2e'", string(out))
	}
}

func TestFinalizeRecordsIngestRowComplete(t *testing.T) {
	v, db := freshVault(t)
	writeInbox(t, v.InboxPath(), "# Inbox\n---\nx\n")
	if _, err := PrepareIngest(v, db); err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeIngest(v, db, ""); err != nil {
		t.Fatal(err)
	}
	// No row in in_progress now.
	if _, err := db.CurrentIngest(); err == nil {
		t.Error("CurrentIngest returned a row after finalize; should be none")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@sparks.local")
	run("config", "user.name", "Sparks Test")
	run("config", "commit.gpgsign", "false")
	// The vault init already dropped files; commit them so the repo has a baseline.
	run("add", "-A")
	run("commit", "-m", "init vault")
	// Ensure sparks.db isn't left as a concurrent-lock-holder — close not needed,
	// modernc.org/sqlite handles this, but if we ever see weird CI flakes this is
	// the place to look.
	_ = filepath.Join(dir, "sparks.db")
	_ = os.Stat
}
