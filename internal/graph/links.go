// Package graph parses and resolves wikilinks across a Sparks vault.
//
// Wikilinks use the `[[Target]]`, `[[Target|display]]`, or `[[Target#section]]`
// syntax. We extract the Target portion and ignore the rest — display text
// is purely presentational, section anchors are intra-page navigation and
// don't change resolution.
//
// Resolution order: exact title match (case-insensitive), then alias match
// (case-insensitive), then broken. The resolver never invents pages.
package graph

import (
	"regexp"
	"strings"
)

// wikilinkRE matches [[Target]], [[Target|display]], or [[Target#anchor]].
// The capture group picks up only the Target, trimmed of leading/trailing
// whitespace by callers. We allow hashes and pipes inside the brackets
// but strip them from the target.
var wikilinkRE = regexp.MustCompile(`\[\[([^\]]+?)\]\]`)

// ExtractLinks returns the distinct targets referenced by `[[...]]` spans
// in body. Order is first-appearance; duplicates are removed so callers
// can count unique references per page.
//
// We strip fenced code blocks before matching so `[[Target]]` inside a
// code example isn't treated as a live link. This is a pedestrian-first
// pass — it won't catch every pathological case (nested fences,
// indented code blocks) but it covers the common ones.
func ExtractLinks(body string) []string {
	body = stripCodeFences(body)
	matches := wikilinkRE.FindAllStringSubmatch(body, -1)
	seen := make(map[string]bool, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		target := normalizeTarget(m[1])
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

// normalizeTarget strips any section anchor, display alias, and
// surrounding whitespace from a raw wikilink body.
func normalizeTarget(raw string) string {
	t := raw
	if i := strings.Index(t, "|"); i >= 0 {
		t = t[:i]
	}
	if i := strings.Index(t, "#"); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}

// stripCodeFences removes ```...``` blocks from body so wikilink regex
// doesn't match things inside code examples. Caller-facing line numbers
// aren't preserved, but this package doesn't report line numbers — lint
// reads lines directly when it needs them.
func stripCodeFences(body string) string {
	lines := strings.Split(body, "\n")
	var out strings.Builder
	inFence := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
