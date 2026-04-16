// Package view is Sparks's built-in HTTP wiki viewer.
//
// Design goals (locked at /office-hours 2026-04-15):
//
//   - Typography-first minimalist aesthetic — narrow measure, serif body,
//     restrained color. Reading experience IS the feature.
//   - Read-only. Edits happen in your editor or via the agent; this
//     surface is for browsing.
//   - Lightweight — zero JS, minimal embedded CSS, no external fonts.
//   - Non-intrusive — ships inside the sparks binary, off by default,
//     launched via `sparks view`.
//
// Wikilink resolution reuses internal/graph.Resolver so the same
// rules that power lint/manifest apply here: title → alias → filename.
package view

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/yogirk/sparks/internal/frontmatter"
	"github.com/yogirk/sparks/internal/graph"
)

// md is the shared markdown processor. GFM for tables + task lists;
// WithUnsafe so literal HTML in notes survives (personal vault, not
// untrusted input).
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// wikilinkRE matches [[Target]], [[Target|display]], and [[Target#anchor]].
// Mirrors the regex in internal/graph/links.go but strips the outer
// brackets here so the replacement emits standard markdown link syntax
// that goldmark then renders. We run this BEFORE goldmark so code
// fences and inline code that already contain `[[...]]` are protected
// by being inside code spans — goldmark won't parse their contents as
// links. We additionally skip wikilinks inside fenced code blocks to
// be safe.
var wikilinkRE = regexp.MustCompile(`\[\[([^\]]+?)\]\]`)

// Resolved represents a single wikilink that went through the resolver.
// Exported so the renderer can report broken links to the template layer
// if we ever want a per-page broken-link count.
type Resolved struct {
	Target   string // what was in the [[...]]
	Display  string // what the user sees (Target by default, or after |)
	Href     string // resolved URL path, or "" if broken
	IsBroken bool
}

// RenderPage converts a markdown body to HTML, substituting wikilinks
// via the supplied resolver. Returns the HTML string and the list of
// resolved links for diagnostic use.
//
// The caller is expected to pass a body with frontmatter already
// stripped. Use PreparePage for the combined flow.
func RenderPage(body string, resolver *graph.Resolver) (string, []Resolved, error) {
	processed, links := rewriteWikilinks(body, resolver)
	var buf bytes.Buffer
	if err := md.Convert([]byte(processed), &buf); err != nil {
		return "", links, fmt.Errorf("render markdown: %w", err)
	}
	return buf.String(), links, nil
}

// PreparePage parses YAML frontmatter off a raw markdown file, then
// renders the remaining body. Returns the parsed frontmatter, the HTML,
// and the resolved links.
//
// The template always emits an <h1>{{.Title}}</h1>, so we strip a
// leading level-1 heading from the body to avoid visible duplication.
func PreparePage(raw []byte, resolver *graph.Resolver) (frontmatter.Frontmatter, string, []Resolved, error) {
	fm, body, err := frontmatter.ParseBytes(raw)
	if err != nil {
		// No frontmatter is OK here — render as plain markdown. The
		// viewer is read-friendly, not schema-strict; lint is where
		// schema violations surface.
		clean := stripLeadingH1(string(raw))
		html, links, rErr := RenderPage(clean, resolver)
		return frontmatter.Frontmatter{}, html, links, rErr
	}
	clean := stripLeadingH1(string(body))
	html, links, rErr := RenderPage(clean, resolver)
	return fm, html, links, rErr
}

// stripLeadingH1 removes a single `# Heading` line at the top of body
// (allowing blank lines and a BOM before it). The template supplies the
// page title h1, and most of our wiki pages also start with `# Title`.
func stripLeadingH1(body string) string {
	trimmed := strings.TrimLeft(body, "\ufeff\r\n\t ")
	if !strings.HasPrefix(trimmed, "# ") {
		return body
	}
	// Find the end of the heading line; drop it and any immediately
	// following blank lines so the rendered body starts cleanly.
	nl := strings.IndexByte(trimmed, '\n')
	if nl < 0 {
		return "" // entire body was just "# Foo"
	}
	rest := trimmed[nl+1:]
	rest = strings.TrimLeft(rest, "\r\n")
	return rest
}

// rewriteWikilinks replaces [[Target]] / [[Target|display]] / [[Target#anchor]]
// with standard markdown links. Broken links become a <span> with a
// "broken-link" class so the CSS can style them distinctly.
//
// Fenced code blocks (lines between ``` markers) are left alone — users
// often paste wikilink-shaped text into code examples and don't want
// them resolved.
func rewriteWikilinks(body string, resolver *graph.Resolver) (string, []Resolved) {
	var out strings.Builder
	out.Grow(len(body) + 128)
	var links []Resolved

	lines := strings.Split(body, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			out.WriteString(line)
			if i < len(lines)-1 {
				out.WriteByte('\n')
			}
			continue
		}
		if inFence {
			out.WriteString(line)
			if i < len(lines)-1 {
				out.WriteByte('\n')
			}
			continue
		}

		rewritten := wikilinkRE.ReplaceAllStringFunc(line, func(match string) string {
			sub := wikilinkRE.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			raw := sub[1]
			target, display := splitDisplay(raw)
			anchor := ""
			if i := strings.Index(target, "#"); i >= 0 {
				anchor = target[i:] // keep the leading '#'
				target = target[:i]
			}
			target = strings.TrimSpace(target)
			if display == "" {
				display = target
				if anchor != "" {
					display = display + anchor
				}
			}

			resolved := resolver.Resolve(target)
			r := Resolved{Target: target, Display: display}
			if resolved == "" {
				r.IsBroken = true
				links = append(links, r)
				// Emit raw HTML that survives goldmark via WithUnsafe.
				// Both attr value and body are escaped so nested quotes
				// in the target text (e.g. 'Some "quoted" thing') don't
				// produce malformed HTML that goldmark then escapes.
				return fmt.Sprintf(`<span class="broken-link" title="no page matches: %s">%s</span>`,
					htmlEscape(target), htmlEscape(display))
			}
			href := pathToHref(resolved) + anchor
			r.Href = href
			links = append(links, r)
			return fmt.Sprintf("[%s](%s)", display, href)
		})
		out.WriteString(rewritten)
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String(), links
}

// splitDisplay peels the `|display` suffix off a wikilink body if present.
func splitDisplay(raw string) (target, display string) {
	if i := strings.Index(raw, "|"); i >= 0 {
		return raw[:i], strings.TrimSpace(raw[i+1:])
	}
	return raw, ""
}

// pathToHref turns a vault-relative manifest path into a URL path under
// the viewer. "wiki/entities/Cascade.md" → "/wiki/entities/Cascade".
// "wiki/entities/Claude Code for BigQuery.md" →
// "/wiki/entities/Claude%20Code%20for%20BigQuery".
//
// The .md suffix is stripped for readability. Each path segment is
// URL-encoded individually so spaces and other special characters in
// wiki filenames survive the browser's URL parser. Using PathEscape
// (not QueryEscape) so `/` separators stay as-is.
func pathToHref(manifestPath string) string {
	p := strings.TrimSuffix(manifestPath, ".md")
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/" + strings.Join(segments, "/")
}

// htmlEscape is a tiny escaper for the broken-link span. We avoid
// html/template here because the caller is injecting raw HTML into
// markdown pre-parse; it's our job to keep it clean.
func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}
