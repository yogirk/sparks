package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/frontmatter"
)

func newFmtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fmt [glob]",
		Short: "Validate frontmatter across wiki pages",
		Long: `Walks wiki/*.md files (or the given glob relative to vault root),
parses frontmatter, and reports schema violations. Use --check in hooks:
any issue returns a non-zero exit code.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runFmt,
	}
	cmd.Flags().Bool("check", false, "non-zero exit on any issue, do not fix")
	return cmd
}

func runFmt(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()

	pattern := filepath.Join(v.Root, "wiki", "**", "*.md")
	if len(args) == 1 {
		pattern = filepath.Join(v.Root, args[0])
	}
	files, err := expandGlob(pattern)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	total := 0
	for _, f := range files {
		issues := validateFile(f)
		if len(issues) == 0 {
			continue
		}
		total += len(issues)
		rel, _ := filepath.Rel(v.Root, f)
		fmt.Fprintf(out, "%s:\n", rel)
		for _, iss := range issues {
			fmt.Fprintf(out, "  - %s\n", iss)
		}
	}
	if total == 0 {
		fmt.Fprintln(out, "Frontmatter OK.")
		return nil
	}
	check, _ := cmd.Flags().GetBool("check")
	_ = check // always non-zero on issues; --check is kept for future
	// asymmetry if `--fix` ships later.
	fmt.Fprintf(out, "\n%d frontmatter issue(s) across %d file(s).\n", total, countAffected(out, files, v.Root))
	return fmt.Errorf("%d frontmatter issue(s)", total)
}

// validateFile parses a markdown file's frontmatter and returns issues.
// Files without frontmatter return a single issue; files outside wiki/
// are skipped (raw/ has no schema).
func validateFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"read error: " + err.Error()}
	}
	fm, _, err := frontmatter.ParseBytes(data)
	if err != nil {
		return []string{"no frontmatter block"}
	}
	return frontmatter.Validate(fm)
}

// expandGlob mimics doublestar behavior enough for our paths: **/*.md
// under wiki/. Standard filepath.Glob doesn't support **, so we walk.
func expandGlob(pattern string) ([]string, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(pattern)
	}
	// Split pattern into base and suffix around the first **.
	idx := strings.Index(pattern, "**")
	base := strings.TrimSuffix(pattern[:idx], string(filepath.Separator))
	suffix := strings.TrimPrefix(pattern[idx+2:], string(filepath.Separator))
	// Walk base, match suffix via filepath.Match on the tail.
	var out []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Match just the final segment pattern (e.g. "*.md").
		ok, _ := filepath.Match(filepath.Base(suffix), filepath.Base(path))
		if ok {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// countAffected returns how many files in the input list contributed to
// total output. io.Writer is unused — it's here so future expansions can
// stream progress without reshaping the signature.
func countAffected(_ io.Writer, files []string, root string) int {
	seen := map[string]bool{}
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		if validateFile(f) != nil && len(validateFile(f)) > 0 {
			seen[rel] = true
		}
	}
	return len(seen)
}
