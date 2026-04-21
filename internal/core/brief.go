package core

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yogirk/sparks/internal/lint"
	"github.com/yogirk/sparks/internal/manifest"
	"github.com/yogirk/sparks/internal/vault"
)

// DefaultBriefDays is the window used when the caller passes 0.
const DefaultBriefDays = 7

// briefTasksPreview caps the number of open tasks echoed in the preview
// slice. The full count is always reported separately.
const briefTasksPreview = 5

// Brief assembles the plumbing snapshot that `sparks brief` emits.
// Pure gathering: every field is derived from the manifest or the
// filesystem. Synthesis stays with the agent.
func Brief(v *vault.Vault, db *manifest.DB, days int) (BriefReport, error) {
	if days <= 0 {
		days = DefaultBriefDays
	}
	now := time.Now()
	since := now.AddDate(0, 0, -days)

	r := BriefReport{
		VaultRoot: v.Root,
		Window:    BriefWindow{Days: days, Since: since, Now: now},
	}

	logDays, err := parseLogWindow(filepath.Join(v.Root, "wiki", "log.md"), since)
	if err != nil {
		return r, err
	}
	r.LogEntries = logDays

	newRaw, err := db.FilesSince("raw/", since)
	if err != nil {
		return r, err
	}
	r.NewRaw = filterBriefRaw(newRaw)

	pages, err := db.ListPages()
	if err != nil {
		return r, err
	}
	r.UpdatedWiki = updatedWikiSince(pages, since)

	r.Revisit = briefRevisit(v, db, pages)

	tasks, err := openTasksPreview(v)
	if err != nil {
		return r, err
	}
	r.Tasks = tasks

	return r, nil
}

// logHeadingRE matches `## [YYYY-MM-DD] ...` — the log.md convention
// documented in the contract ingest section.
var logHeadingRE = regexp.MustCompile(`^##\s+\[(\d{4}-\d{2}-\d{2})\]`)

// parseLogWindow scans wiki/log.md top-to-bottom, chunks it by `## [YYYY-MM-DD]`
// headings, and returns the blocks whose date is at or after since. A
// missing log file is not an error — the vault may never have been
// ingested. Order is most-recent-first.
func parseLogWindow(path string, since time.Time) ([]BriefLogDay, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	// Compare as YYYY-MM-DD strings so the window boundary aligns with
	// calendar dates in the user's local tz. Parsing `2026-04-14` with
	// time.Parse returns UTC, which would misclassify by a day for
	// users in negative offsets.
	cutoff := startOfDay(since).Format("2006-01-02")
	var (
		out          []BriefLogDay
		currentDate  string
		currentLines []string
	)
	flush := func() {
		if currentDate == "" {
			return
		}
		if currentDate < cutoff {
			return
		}
		text := strings.TrimRight(strings.Join(currentLines, "\n"), "\n")
		out = append(out, BriefLogDay{Date: currentDate, Text: text})
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if m := logHeadingRE.FindStringSubmatch(line); m != nil {
			flush()
			currentDate = m[1]
			currentLines = []string{line}
			continue
		}
		if currentDate != "" {
			currentLines = append(currentLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()

	sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, nil
}

// filterBriefRaw converts manifest rows to BriefFile, dropping paths
// that represent the auto-archive and the tasks capture dir (both are
// CLI-owned, not human signal).
func filterBriefRaw(files []manifest.FileRecord) []BriefFile {
	out := make([]BriefFile, 0, len(files))
	for _, fr := range files {
		if strings.HasPrefix(fr.Path, "raw/inbox/") {
			continue
		}
		if strings.HasPrefix(fr.Path, "raw/archive/") {
			continue
		}
		out = append(out, BriefFile{Path: fr.Path, MTime: fr.MTime})
	}
	return out
}

// updatedWikiSince filters pages whose frontmatter updated: date is at
// or after the window start. Pages without a parseable updated: date
// are skipped — lint catches those separately.
func updatedWikiSince(pages []manifest.PageInfo, since time.Time) []BriefPage {
	cutoff := startOfDay(since).Format("2006-01-02")
	var out []BriefPage
	for _, p := range pages {
		if p.Updated == "" {
			continue
		}
		// Guard against malformed dates: ensure YYYY-MM-DD is parseable
		// before doing a lexicographic compare.
		if _, err := time.Parse("2006-01-02", p.Updated); err != nil {
			continue
		}
		if p.Updated < cutoff {
			continue
		}
		out = append(out, BriefPage{
			Path:     p.Path,
			Title:    p.Title,
			Type:     p.Type,
			Maturity: p.Maturity,
			Updated:  p.Updated,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Updated != out[j].Updated {
			return out[i].Updated > out[j].Updated
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// briefRevisit rolls up "worth a second look" signals. Stale and thin
// reuse the lint pass so brief and lint agree by construction. Seed
// orphans are computed here — lint flags orphans of every maturity,
// but for a weekly synthesis we only want the seed ones (young pages
// that nothing links to yet).
func briefRevisit(v *vault.Vault, db *manifest.DB, pages []manifest.PageInfo) BriefRevisit {
	var r BriefRevisit
	if rep, err := lint.Run(v, db); err == nil {
		for _, iss := range rep.Issues {
			switch iss.Check {
			case "stale-pages":
				r.Stale = append(r.Stale, iss.Path)
			case "thin-pages":
				r.Thin = append(r.Thin, iss.Path)
			}
		}
	}

	for _, p := range pages {
		if p.Maturity != "seed" {
			continue
		}
		if filepath.Base(p.Path) == "index.md" {
			continue
		}
		incoming, err := db.IncomingLinks(p.Path)
		if err != nil {
			continue
		}
		real := 0
		for _, e := range incoming {
			if filepath.Base(e.Source) == "index.md" {
				continue
			}
			real++
		}
		if real == 0 {
			r.SeedOrphans = append(r.SeedOrphans, p.Path)
		}
	}
	sort.Strings(r.Stale)
	sort.Strings(r.Thin)
	sort.Strings(r.SeedOrphans)
	return r
}

// openTasksPreview parses wiki/collections/Tasks.md for `- [ ]` lines and
// returns the count plus a short preview. Mirrors how `sparks done` finds
// tasks but without the fuzzy-match layer.
func openTasksPreview(v *vault.Vault) (BriefTasks, error) {
	path := filepath.Join(v.Root, TasksFilename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BriefTasks{}, nil
		}
		return BriefTasks{}, err
	}
	defer f.Close()

	var t BriefTasks
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "- [ ]") {
			continue
		}
		t.Open++
		if len(t.First) < briefTasksPreview {
			text := strings.TrimSpace(strings.TrimPrefix(line, "- [ ]"))
			t.First = append(t.First, text)
		}
	}
	if err := scanner.Err(); err != nil {
		return BriefTasks{}, err
	}
	return t, nil
}

// startOfDay drops the time-of-day so a window boundary lines up with
// calendar dates. Users asking for "the last 7 days" mean the last 7
// calendar days, not a rolling 168-hour window.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
