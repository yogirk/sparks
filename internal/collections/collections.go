// Package collections regenerates the auto-derived index pages under
// wiki/collections/. Every collection is a deterministic function of
// raw/ files plus, in some cases, the manifest. The Tasks collection
// is the live exception and is NOT handled here.
//
// Adding a new collection type means adding an Extractor entry to the
// Registry and a corresponding Default in DefaultGlobs. Callers don't
// pick types — they call Regenerate which iterates the registry.
package collections

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// Extractor is the contract every collection implements: read source
// files (typically under raw/), produce the markdown body that should
// land at wiki/collections/{Name}.md.
//
// glob is either the user override from sparks.toml or the package
// default. Extractors that derive from non-glob sources (Books,
// ReadingList, Projects) ignore it.
type Extractor func(v *vault.Vault, db *manifest.DB, glob string) (body string, err error)

// Registry holds every collection's name → extractor mapping. Order is
// stable thanks to OrderedNames; map iteration is intentionally not
// used for output paths.
var Registry = map[string]Extractor{
	"Quotes":      extractQuotes,
	"Bookmarks":   extractBookmarks,
	"Books":       extractBooks,
	"ReadingList": extractReadingList,
	"Media":       extractMedia,
	"Ideas":       extractIdeas,
	"Projects":    extractProjects,
}

// DefaultGlobs is the source-glob default per collection. Empty string
// means the collection doesn't take a glob (derived from manifest or
// inbox archive). Users can override via [collections.<lowername>] in
// sparks.toml — see vault.Config.Collections.
var DefaultGlobs = map[string]string{
	"Quotes":      "raw/quotes/**/*.md",
	"Bookmarks":   "raw/weblinks/**/*.md",
	"Books":       "",
	"ReadingList": "",
	"Media":       "raw/media/**/*.md",
	"Ideas":       "raw/ideas/**/*.md",
	"Projects":    "",
}

// OutputFilename returns the wiki/collections/X.md path for a collection.
// Built from a literal map so renames are explicit, not derived from
// case rules someone has to guess.
var OutputFilename = map[string]string{
	"Quotes":      "Quotes.md",
	"Bookmarks":   "Bookmarks.md",
	"Books":       "Books.md",
	"ReadingList": "Reading List.md",
	"Media":       "Media.md",
	"Ideas":       "Ideas.md",
	"Projects":    "Projects.md",
}

// OrderedNames is the deterministic processing/display order. Used by
// the CLI summary so output doesn't shuffle between runs.
var OrderedNames = []string{
	"Quotes",
	"Bookmarks",
	"Books",
	"ReadingList",
	"Media",
	"Ideas",
	"Projects",
}

// Result is what Regenerate returns per collection.
type Result struct {
	Name        string `json:"name"`
	OutputPath  string `json:"output_path"`
	Bytes       int    `json:"bytes"`
	Skipped     bool   `json:"skipped,omitempty"`
	SkipReason  string `json:"skip_reason,omitempty"`
	Error       string `json:"error,omitempty"`
}

// RegenerateOptions selects what runs. If Names is non-empty, only those
// collections regenerate. DryRun reports what would change without
// writing.
type RegenerateOptions struct {
	Names  []string
	DryRun bool
}

// Regenerate walks the requested collections and writes each output.
// Tasks is implicitly excluded — it's a live page, not derived.
func Regenerate(v *vault.Vault, db *manifest.DB, opts RegenerateOptions) ([]Result, error) {
	names := opts.Names
	if len(names) == 0 {
		names = OrderedNames
	}
	outDir := filepath.Join(v.Root, "wiki", "collections")
	if !opts.DryRun {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir collections: %w", err)
		}
	}

	results := make([]Result, 0, len(names))
	for _, name := range names {
		extractor, ok := Registry[name]
		if !ok {
			results = append(results, Result{Name: name, Error: "unknown collection"})
			continue
		}
		glob := chooseGlob(v, name)
		body, err := extractor(v, db, glob)
		if err != nil {
			results = append(results, Result{Name: name, Error: err.Error()})
			continue
		}
		path := filepath.Join(outDir, OutputFilename[name])
		if opts.DryRun {
			results = append(results, Result{Name: name, OutputPath: path, Bytes: len(body)})
			continue
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return results, fmt.Errorf("write %s: %w", path, err)
		}
		results = append(results, Result{Name: name, OutputPath: path, Bytes: len(body)})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return indexOf(OrderedNames, results[i].Name) < indexOf(OrderedNames, results[j].Name)
	})
	return results, nil
}

// chooseGlob returns the user override from sparks.toml if present and
// the collection allows overriding, otherwise the package default.
func chooseGlob(v *vault.Vault, name string) string {
	if v.Config != nil {
		// Map keys in TOML are typically lowercase.
		key := lowerName(name)
		if override, ok := v.Config.Collections[key]; ok && override.Glob != "" {
			return override.Glob
		}
	}
	return DefaultGlobs[name]
}

func lowerName(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
