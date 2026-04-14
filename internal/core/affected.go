package core

import (
	"sort"
	"strings"

	"github.com/yogirk/sparks/internal/collections"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// AffectedResult reports which collections need regeneration based on
// files changed since the last finalized ingest.
type AffectedResult struct {
	Affected []string            `json:"affected"`
	Reason   map[string][]string `json:"reason"` // collection -> paths that triggered it
}

// Affected scans the manifest for files modified since the most recent
// completed ingest and maps them to the collections whose source globs
// they touch. Tasks is never returned (live page, not regenerated).
func Affected(v *vault.Vault, db *manifest.DB) (AffectedResult, error) {
	changed, err := db.ChangedSinceLastIngest()
	if err != nil {
		return AffectedResult{}, err
	}
	if len(changed) == 0 {
		return AffectedResult{Affected: []string{}, Reason: map[string][]string{}}, nil
	}

	reason := map[string][]string{}
	for _, name := range collections.OrderedNames {
		glob := chooseAffectedGlob(v, name)
		if glob == "" {
			// Derived collections (Books, ReadingList, Projects) recompute
			// off raw/inbox/* and the manifest. Conservative: trigger them
			// any time any inbox archive changes.
			if name == "Projects" {
				if anyMatches(changed, func(p string) bool { return strings.HasPrefix(p, "wiki/entities/") }) {
					reason[name] = append(reason[name], "wiki/entities/* changed")
				}
				continue
			}
			if anyMatches(changed, func(p string) bool { return strings.HasPrefix(p, "raw/inbox/") }) {
				reason[name] = []string{"raw/inbox/* changed"}
			}
			continue
		}
		prefix := strings.TrimSuffix(strings.SplitN(glob, "**", 2)[0], "/")
		var hits []string
		for _, p := range changed {
			if strings.HasPrefix(p, prefix+"/") || p == prefix {
				hits = append(hits, p)
			}
		}
		if len(hits) > 0 {
			reason[name] = hits
		}
	}

	affected := make([]string, 0, len(reason))
	for k := range reason {
		affected = append(affected, k)
	}
	sort.Strings(affected)
	return AffectedResult{Affected: affected, Reason: reason}, nil
}

// chooseAffectedGlob mirrors collections.chooseGlob but stays in core to
// avoid exporting the unexported helper. Tiny duplication, intentional.
func chooseAffectedGlob(v *vault.Vault, name string) string {
	if v.Config != nil {
		key := strings.ToLower(name)
		if override, ok := v.Config.Collections[key]; ok && override.Glob != "" {
			return override.Glob
		}
	}
	return collections.DefaultGlobs[name]
}

func anyMatches(paths []string, pred func(string) bool) bool {
	for _, p := range paths {
		if pred(p) {
			return true
		}
	}
	return false
}
