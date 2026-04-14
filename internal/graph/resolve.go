package graph

import (
	"strings"
)

// PageRef is the minimum a resolver needs to know about a vault page:
// its canonical path and the names by which it can be looked up.
type PageRef struct {
	Path    string
	Title   string
	Aliases []string
}

// Resolver answers "what path does this wikilink target point to?" using
// a pre-built index keyed by case-folded title, alias, and filename.
// Keeping the index in memory is fine for personal-scale vaults (a few
// thousand pages at most).
//
// Resolution order: title → alias → filename. The filename fallback
// matches Obsidian's convention — `[[Project Trace]]` resolves to
// `wiki/entities/Project Trace.md` even when the page's frontmatter
// title is something more verbose like
// `"Project Trace - Application-Centric Cloud Dashboard"`.
type Resolver struct {
	byTitle    map[string]string // folded title -> path
	byAlias    map[string]string // folded alias -> path
	byFilename map[string]string // folded filename (no .md) -> path
}

// NewResolver builds a lookup over the given page set. Duplicate titles,
// aliases, or filenames silently keep the first-inserted page;
// duplicate-alias detection is the lint package's job, not the resolver's.
func NewResolver(pages []PageRef) *Resolver {
	r := &Resolver{
		byTitle:    make(map[string]string, len(pages)),
		byAlias:    make(map[string]string, len(pages)),
		byFilename: make(map[string]string, len(pages)),
	}
	for _, p := range pages {
		if p.Title != "" {
			key := strings.ToLower(strings.TrimSpace(p.Title))
			if _, exists := r.byTitle[key]; !exists {
				r.byTitle[key] = p.Path
			}
		}
		for _, a := range p.Aliases {
			key := strings.ToLower(strings.TrimSpace(a))
			if key == "" {
				continue
			}
			if _, exists := r.byAlias[key]; !exists {
				r.byAlias[key] = p.Path
			}
		}
		if name := filenameKey(p.Path); name != "" {
			if _, exists := r.byFilename[name]; !exists {
				r.byFilename[name] = p.Path
			}
		}
	}
	return r
}

// Resolve returns the path pointed at by target, or the empty string if
// the link is broken. Matches title, then alias, then filename basename.
// Case-insensitive at every step.
func (r *Resolver) Resolve(target string) string {
	key := strings.ToLower(strings.TrimSpace(target))
	if key == "" {
		return ""
	}
	if p, ok := r.byTitle[key]; ok {
		return p
	}
	if p, ok := r.byAlias[key]; ok {
		return p
	}
	if p, ok := r.byFilename[key]; ok {
		return p
	}
	return ""
}

// filenameKey returns the case-folded basename of path with `.md` stripped,
// suitable for use as a Resolver lookup key. Empty if the path doesn't
// look like a markdown file.
func filenameKey(path string) string {
	last := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			last = i
			break
		}
	}
	base := path[last+1:]
	if !strings.HasSuffix(strings.ToLower(base), ".md") {
		return ""
	}
	base = base[:len(base)-3]
	return strings.ToLower(strings.TrimSpace(base))
}

// ResolveOrTarget returns the resolved path, or the original target if
// broken. Useful for manifest writes where we want to record an
// unresolved target rather than drop it.
func (r *Resolver) ResolveOrTarget(target string) string {
	if p := r.Resolve(target); p != "" {
		return p
	}
	return strings.TrimSpace(target)
}
