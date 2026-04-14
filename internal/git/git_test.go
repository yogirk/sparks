package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAssertAvailable(t *testing.T) {
	// Assumes git is installed on the test host (reasonable for Sparks
	// developers). If it's missing, the test harness has bigger problems.
	if err := AssertAvailable(); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
}

func TestIsRepoFalseForPlainDir(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("IsRepo on plain tempdir = true, want false")
	}
}

func TestCommitIfDirtyErrorsOnNonRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := CommitIfDirty(dir, "test")
	if !errors.Is(err, ErrNotRepo) {
		t.Errorf("err = %v, want ErrNotRepo", err)
	}
}

func TestCommitIfDirtyNoopWhenClean(t *testing.T) {
	dir := initRepo(t)
	sha, err := CommitIfDirty(dir, "noop")
	if err != nil {
		t.Fatalf("CommitIfDirty clean repo: %v", err)
	}
	if sha != "" {
		t.Errorf("clean repo commit returned sha %q, want empty", sha)
	}
}

func TestCommitIfDirtyStagesAndCommits(t *testing.T) {
	dir := initRepo(t)
	file := filepath.Join(dir, "note.md")
	if err := os.WriteFile(file, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := CommitIfDirty(dir, "add note")
	if err != nil {
		t.Fatalf("CommitIfDirty: %v", err)
	}
	if len(sha) < 7 {
		t.Errorf("sha = %q, want short hash (>=7 chars)", sha)
	}
	// Repo should now be clean.
	sha2, err := CommitIfDirty(dir, "nothing")
	if err != nil || sha2 != "" {
		t.Errorf("second commit on clean repo: sha=%q err=%v", sha2, err)
	}
}

// initRepo creates a git repo in a fresh tempdir with a local identity
// configured so commits don't fail on CI runners without global config.
func initRepo(t *testing.T) string {
	t.Helper()
	if err := AssertAvailable(); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
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
	return dir
}
