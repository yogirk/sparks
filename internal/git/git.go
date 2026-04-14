// Package git wraps the system git binary via os/exec. Sparks uses this
// for the small set of operations needed during ingest finalization
// (status check, stage, commit). We deliberately shell out instead of
// embedding go-git: the user's git config, hooks, and signing all work
// without duplicating logic, and the binary stays small.
//
// Decision reference: /plan-eng-review A2 (2026-04-14).
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Sentinel errors.
var (
	// ErrNotAvailable is returned by AssertAvailable if `git` isn't on PATH.
	ErrNotAvailable = errors.New("git: binary not found on PATH")
	// ErrNotRepo signals the directory is not inside a git working tree.
	// Callers typically warn and continue (commits are optional).
	ErrNotRepo = errors.New("git: not a git repository")
)

// AssertAvailable verifies `git` is executable. Call once at startup so
// failures surface immediately, not halfway through an ingest.
func AssertAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrNotAvailable
	}
	return nil
}

// IsRepo reports whether dir is inside a git working tree. Returns false
// without error for non-repos — this is a question, not an assertion.
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// CommitIfDirty stages everything in dir and commits with message.
// Returns the new commit SHA, or empty string if there was nothing to
// commit (working tree was clean). ErrNotRepo if dir is not a git repo.
//
// Commit identity follows the repo's git config — Sparks never sets or
// overrides user.email. If you want a separate identity for vault commits,
// run `git config --local user.email "..."` yourself.
func CommitIfDirty(dir, message string) (string, error) {
	if !IsRepo(dir) {
		return "", ErrNotRepo
	}
	dirty, err := hasChanges(dir)
	if err != nil {
		return "", err
	}
	if !dirty {
		return "", nil
	}
	if err := run(dir, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}
	if err := run(dir, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	sha, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(sha)), nil
}

// hasChanges reports whether there are staged, unstaged, or untracked
// changes relative to HEAD. Uses --porcelain so output is stable across
// git versions.
func hasChanges(dir string) (bool, error) {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

// run executes `git -C dir args...`, discarding stdout and wrapping any
// error with the combined stderr for a readable failure message.
func run(dir string, args ...string) error {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
