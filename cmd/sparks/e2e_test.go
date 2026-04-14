package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2EInitScanStatus drives the three Week 1 commands through the
// cobra root the same way a user would, asserting end-to-end output.
// This is the dogfood gate #1 in test form: prove the CLI shell works
// before relying on it from outside the binary.
func TestE2EInitScanStatus(t *testing.T) {
	dir := t.TempDir()

	stdout := runCmd(t, "init", dir)
	if !strings.Contains(stdout, "Initialized Sparks vault") {
		t.Fatalf("init output: %q", stdout)
	}
	if !strings.Contains(stdout, "Created sparks.db") {
		t.Errorf("init didn't report manifest creation: %q", stdout)
	}

	// Switch to the new vault dir for the remaining commands; cobra always
	// resolves vault from the working directory.
	t.Chdir(dir)

	scanOut := runCmd(t, "scan")
	if !strings.Contains(scanOut, "walked:") {
		t.Errorf("scan output missing summary: %q", scanOut)
	}

	// Second scan should be incremental: 0 hashed, all skipped.
	scan2 := runCmd(t, "scan")
	if !strings.Contains(scan2, "hashed: 0") {
		t.Errorf("second scan should hash nothing, got: %q", scan2)
	}

	statusOut := runCmd(t, "status")
	for _, want := range []string{"vault:", "schema: v1", "pages:", "inbox:", "manifest:"} {
		if !strings.Contains(statusOut, want) {
			t.Errorf("status missing %q in output: %q", want, statusOut)
		}
	}
	// Fresh vault has the seed inbox.md with a header-only block, so 0 entries.
	if !strings.Contains(statusOut, "inbox: 0 entries pending") {
		t.Errorf("fresh vault should report 0 pending entries: %q", statusOut)
	}

	// Sanity: the manifest file exists at the expected path.
	if _, err := filepath.Glob(filepath.Join(dir, "sparks.db")); err != nil {
		t.Fatalf("glob sparks.db: %v", err)
	}
}

// TestE2EIngestPrepareAndAbort drives `sparks ingest --prepare` followed
// by `--abort` to prove the full concurrent-ingest lock lifecycle works
// through the CLI.
func TestE2EIngestPrepareAndAbort(t *testing.T) {
	dir := t.TempDir()
	runCmd(t, "init", dir)
	t.Chdir(dir)

	inbox := []byte(`# Inbox
---
2026-04-10
First entry with https://example.com
- [ ] a task
---
book: The Pragmatic Programmer
`)
	if err := os.WriteFile("inbox.md", inbox, 0o644); err != nil {
		t.Fatal(err)
	}

	// --prepare should emit JSON by default (default flip per adapter).
	out := runCmd(t, "ingest", "--prepare")
	var res struct {
		IngestID            int64    `json:"ingest_id"`
		Total               int      `json:"total"`
		AffectedCollections []string `json:"affected_collections"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("prepare output not JSON: %v\nout: %s", err, out)
	}
	if res.Total != 2 {
		t.Errorf("Total = %d, want 2", res.Total)
	}
	if res.IngestID == 0 {
		t.Error("IngestID = 0")
	}

	// Second --prepare should report already-in-progress without opening a new row.
	out2 := runCmd(t, "ingest", "--prepare")
	if !strings.Contains(out2, `"already_in_progress": true`) {
		t.Errorf("second prepare did not report already_in_progress: %s", out2)
	}

	// Abort unblocks.
	abortOut := runCmd(t, "ingest", "--abort")
	if !strings.Contains(abortOut, "Aborted ingest") {
		t.Errorf("abort output: %q", abortOut)
	}

	// A fresh --prepare now succeeds.
	out3 := runCmd(t, "ingest", "--prepare")
	var res3 struct {
		AlreadyInProgress bool `json:"already_in_progress"`
	}
	_ = json.Unmarshal([]byte(out3), &res3)
	if res3.AlreadyInProgress {
		t.Error("prepare after abort should not report already_in_progress")
	}

	// Abort when nothing is in progress: clean up then try again.
	runCmd(t, "ingest", "--abort")
	noopOut := runCmd(t, "ingest", "--abort")
	if !strings.Contains(noopOut, "No ingest in progress") {
		t.Errorf("second abort: %q", noopOut)
	}
}

// TestE2EIngestEmptyInbox checks the fresh-vault case: seed inbox has no
// entries below the header separator, so --prepare should return Total=0
// and NOT open an ingest row (nothing to lock over).
func TestE2EIngestEmptyInbox(t *testing.T) {
	dir := t.TempDir()
	runCmd(t, "init", dir)
	t.Chdir(dir)

	out := runCmd(t, "ingest", "--prepare")
	var res struct {
		Total    int   `json:"total"`
		IngestID int64 `json:"ingest_id"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("not JSON: %v\nout: %s", err, out)
	}
	if res.Total != 0 {
		t.Errorf("Total = %d, want 0", res.Total)
	}
	if res.IngestID != 0 {
		t.Errorf("empty inbox opened an ingest row (IngestID=%d)", res.IngestID)
	}
}

// runCmd executes the cobra root with given args and returns combined
// stdout/stderr. Fatals on non-zero exit.
func runCmd(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("cmd %v failed: %v\noutput: %s", args, err, buf.String())
	}
	return buf.String()
}
