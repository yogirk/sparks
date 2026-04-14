package main

import (
	"bytes"
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
