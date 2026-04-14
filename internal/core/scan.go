package core

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yogirk/sparks/internal/frontmatter"
	"github.com/yogirk/sparks/internal/graph"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// scanIgnoreDirs are directories whose contents we never track in the
// manifest. The vault git repo and the manifest itself are noisy and not
// content the agent reasons over.
var scanIgnoreDirs = map[string]bool{
	".git":         true,
	".obsidian":    true,
	".trash":       true,
	"node_modules": true,
}

// Scan walks the vault, updating the manifest with file hashes, sizes, and
// frontmatter. Files whose mtime AND size match the stored record skip the
// re-hash and re-parse — incremental scan. Files no longer present are
// marked deleted (not removed) so historical references stay resolvable.
func Scan(v *vault.Vault, db *manifest.DB) (ScanResult, error) {
	start := time.Now()
	result := ScanResult{VaultRoot: v.Root}

	known, err := db.AllPaths()
	if err != nil {
		return result, err
	}
	seen := make(map[string]bool, len(known))

	walkErr := filepath.WalkDir(v.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ScanError{Path: rel(v.Root, path), Err: err.Error()})
			return nil
		}
		if d.IsDir() {
			if scanIgnoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip the manifest itself and any sparks.toml at root depth.
		base := d.Name()
		if base == vault.ManifestFilename || strings.HasPrefix(base, vault.ManifestFilename+"-") {
			return nil
		}

		relPath := rel(v.Root, path)
		seen[relPath] = true
		result.Walked++

		info, err := d.Info()
		if err != nil {
			result.Errors = append(result.Errors, ScanError{Path: relPath, Err: err.Error()})
			return nil
		}

		existing, getErr := db.GetFile(relPath)
		incremental := getErr == nil &&
			existing.Size == info.Size() &&
			existing.MTime.Equal(info.ModTime())

		if incremental {
			result.Skipped++
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			result.Errors = append(result.Errors, ScanError{Path: relPath, Err: err.Error()})
			return nil
		}

		now := time.Now().UTC()
		fr := manifest.FileRecord{
			Path:     relPath,
			Hash:     hash,
			Size:     info.Size(),
			MTime:    info.ModTime().UTC(),
			ScanTime: now,
		}
		if err := db.UpsertFile(fr); err != nil {
			result.Errors = append(result.Errors, ScanError{Path: relPath, Err: err.Error()})
			return nil
		}
		result.Hashed++

		// Parse frontmatter for wiki markdown only. Raw markdown stays opaque
		// to the manifest; agents own raw semantics.
		if isWikiMarkdown(relPath) {
			if err := refreshFrontmatter(db, relPath, path); err != nil {
				result.Errors = append(result.Errors, ScanError{Path: relPath, Err: err.Error()})
			}
		}
		return nil
	})
	if walkErr != nil {
		return result, walkErr
	}

	// Diff: anything in known but not seen this run is now deleted.
	var disappeared []string
	for _, p := range known {
		if !seen[p] {
			disappeared = append(disappeared, p)
		}
	}
	if len(disappeared) > 0 {
		if err := db.MarkDeleted(disappeared); err != nil {
			return result, err
		}
		result.Deleted = len(disappeared)
	}

	// Second pass: rebuild the wikilink graph now that all frontmatter is
	// in place. We need the full page set to resolve links (title →
	// path, alias → path), so this has to run after phase one finishes.
	if err := rebuildLinks(v, db); err != nil {
		result.Errors = append(result.Errors, ScanError{Path: "<links>", Err: err.Error()})
	}

	result.Duration = time.Since(start)
	return result, nil
}

// rebuildLinks walks every tracked wiki page, extracts `[[...]]` targets
// from its body, resolves them against the full page set, and replaces
// the manifest's wikilink edges for that page. Called as the second
// phase of Scan.
func rebuildLinks(v *vault.Vault, db *manifest.DB) error {
	pages, err := db.ListPages()
	if err != nil {
		return err
	}
	refs := make([]graph.PageRef, 0, len(pages))
	for _, p := range pages {
		refs = append(refs, graph.PageRef{Path: p.Path, Title: p.Title, Aliases: p.Aliases})
	}
	resolver := graph.NewResolver(refs)

	for _, p := range pages {
		body, err := readBody(filepath.Join(v.Root, p.Path))
		if err != nil {
			return err
		}
		targets := graph.ExtractLinks(body)
		edges := make([]manifest.WikilinkEdge, 0, len(targets))
		for _, t := range targets {
			edges = append(edges, manifest.WikilinkEdge{
				Source:   p.Path,
				Target:   t,
				Resolved: resolver.Resolve(t),
			})
		}
		if err := db.ReplaceLinks(p.Path, edges); err != nil {
			return err
		}
	}
	return nil
}

// readBody reads a markdown file and returns the body portion (everything
// after the frontmatter block, or the full file if no frontmatter).
func readBody(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	_, body, err := frontmatter.ParseBytes(data)
	if err != nil {
		// No frontmatter: treat the entire file as body. Wikilinks in a
		// frontmatter-less page are still meaningful to the graph.
		return string(data), nil
	}
	return string(body), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	// Always store forward-slash paths in the manifest so Windows and macOS
	// vaults can be compared and synced via git without confusion.
	return filepath.ToSlash(r)
}

func isWikiMarkdown(relPath string) bool {
	if filepath.Ext(relPath) != ".md" {
		return false
	}
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	return len(parts) > 1 && parts[0] == "wiki"
}

func refreshFrontmatter(db *manifest.DB, relPath, fullPath string) error {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}
	fm, _, err := frontmatter.ParseBytes(data)
	if err != nil {
		// No frontmatter on a wiki page is allowed during scan; lint will
		// flag it. We still want a row so queries by path succeed.
		_ = db.DeleteFrontmatter(relPath)
		return nil
	}
	return db.UpsertFrontmatter(manifest.FrontmatterRecord{
		Path:     relPath,
		Title:    fm.Title,
		Type:     fm.Type,
		Maturity: fm.Maturity,
		Tags:     fm.Tags,
		Aliases:  fm.Aliases,
		Sources:  fm.Sources,
		Created:  fm.Created,
		Updated:  fm.Updated,
	})
}
