package collections

import (
	"os"
	"path/filepath"
	"strings"
)

// expandGlob mirrors fmt.go's globber: handles a single `**` segment
// followed by a basename pattern. Returns absolute paths under root.
//
// Patterns we support:
//
//	"raw/quotes/**/*.md" → walk raw/quotes/, match *.md
//	"raw/media/*.md"     → standard filepath.Glob
//
// Anything fancier (multiple **s, path-spanning patterns) returns an
// empty list; we keep glob expansion small on purpose.
func expandGlob(root, pattern string) ([]string, error) {
	if pattern == "" {
		return nil, nil
	}
	full := filepath.Join(root, pattern)
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(full)
	}
	idx := strings.Index(full, "**")
	base := strings.TrimSuffix(full[:idx], string(filepath.Separator))
	suffix := strings.TrimPrefix(full[idx+2:], string(filepath.Separator))
	tail := filepath.Base(suffix)

	var out []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // missing source dir is a non-error; collection is just empty
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		ok, _ := filepath.Match(tail, filepath.Base(path))
		if ok {
			out = append(out, path)
		}
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

// rel returns the vault-relative path with forward slashes (manifest convention).
func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}
