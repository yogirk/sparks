package view

import (
	"strings"
	"testing"

	"github.com/yogirk/sparks/internal/graph"
)

// newResolverFromPages is a tiny test helper — avoids pulling vault into
// the render tests, which are purely about markdown + link mechanics.
func newResolverFromPages(pages ...graph.PageRef) *graph.Resolver {
	return graph.NewResolver(pages)
}

func TestRenderPageResolvesTitle(t *testing.T) {
	r := newResolverFromPages(
		graph.PageRef{Path: "wiki/entities/Cascade.md", Title: "Cascade"},
	)
	html, _, err := RenderPage("See [[Cascade]] for details.", r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `href="/wiki/entities/Cascade"`) {
		t.Errorf("resolved link missing: %s", html)
	}
}

func TestRenderPageEncodesSpacesInHref(t *testing.T) {
	r := newResolverFromPages(
		graph.PageRef{Path: "wiki/entities/Claude Code for BigQuery.md", Title: "Claude Code for BigQuery"},
	)
	html, _, err := RenderPage("Try [[Claude Code for BigQuery]].", r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "/wiki/entities/Claude%20Code%20for%20BigQuery") {
		t.Errorf("spaces not URL-encoded: %s", html)
	}
	if strings.Contains(html, "Claude Code for BigQuery.md") {
		t.Errorf("href leaked .md suffix: %s", html)
	}
}

func TestRenderPageBrokenLinkStyled(t *testing.T) {
	r := newResolverFromPages() // empty resolver
	html, _, err := RenderPage("Check [[Ghost]].", r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `class="broken-link"`) {
		t.Errorf("broken link not marked: %s", html)
	}
}

func TestRenderPageWikilinkInsideCodeFenceNotResolved(t *testing.T) {
	r := newResolverFromPages(
		graph.PageRef{Path: "wiki/entities/Cascade.md", Title: "Cascade"},
	)
	body := "Before [[Cascade]].\n\n```\nExample: [[Cascade]]\n```\n\nAfter.\n"
	html, _, err := RenderPage(body, r)
	if err != nil {
		t.Fatal(err)
	}
	// First occurrence should resolve to a link.
	if !strings.Contains(html, `href="/wiki/entities/Cascade"`) {
		t.Errorf("real wikilink not resolved: %s", html)
	}
	// The code-fenced one should still have the raw [[...]] text.
	fencedCount := strings.Count(html, "[[Cascade]]")
	if fencedCount < 1 {
		t.Errorf("fenced [[Cascade]] got rewritten; should be preserved verbatim. html: %s", html)
	}
}

func TestRenderPageWithDisplayOverride(t *testing.T) {
	r := newResolverFromPages(
		graph.PageRef{Path: "wiki/entities/Cascade.md", Title: "Cascade"},
	)
	html, _, err := RenderPage("Read [[Cascade|the Cascade page]].", r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, ">the Cascade page</a>") {
		t.Errorf("display override not honored: %s", html)
	}
}

func TestPreparePageStripsLeadingH1(t *testing.T) {
	raw := []byte(`---
title: Cascade
type: entity
maturity: working
sources: [raw/x.md]
created: 2026-04-01
updated: 2026-04-10
---

# Cascade

First paragraph.
`)
	_, html, _, err := PreparePage(raw, graph.NewResolver(nil))
	if err != nil {
		t.Fatal(err)
	}
	// Rendered body should NOT contain "<h1>Cascade</h1>" (the template
	// adds that). It should start with the first paragraph.
	if strings.Contains(html, "<h1") {
		t.Errorf("leading h1 not stripped: %s", html)
	}
	if !strings.Contains(html, "<p>First paragraph.</p>") {
		t.Errorf("body missing first paragraph: %s", html)
	}
}
