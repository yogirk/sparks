package graph

import (
	"strings"
	"testing"
)

func TestExtractLinksBasic(t *testing.T) {
	body := `See [[Cascade]] and [[BigQuery|BQ]] and [[Sparks#what-it-is]].
Repeat of [[Cascade]] should dedupe.
`
	got := ExtractLinks(body)
	want := []string{"Cascade", "BigQuery", "Sparks"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractLinksIgnoresCodeFences(t *testing.T) {
	body := "Outside [[Real]].\n```md\nInside [[Fake]].\n```\nAfter [[AlsoReal]].\n"
	got := ExtractLinks(body)
	if len(got) != 2 {
		t.Errorf("got %v, want [Real AlsoReal] (code-fenced link must be stripped)", got)
	}
	if got[0] != "Real" || got[1] != "AlsoReal" {
		t.Errorf("got %v", got)
	}
}

func TestExtractLinksNoMatchesReturnsEmpty(t *testing.T) {
	got := ExtractLinks("no wikilinks here, just prose.")
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestResolveByTitle(t *testing.T) {
	r := NewResolver([]PageRef{
		{Path: "wiki/entities/Cascade.md", Title: "Cascade"},
	})
	if got := r.Resolve("Cascade"); got != "wiki/entities/Cascade.md" {
		t.Errorf("exact title: got %q", got)
	}
	if got := r.Resolve("cascade"); got != "wiki/entities/Cascade.md" {
		t.Errorf("case-insensitive title: got %q", got)
	}
	if got := r.Resolve("  Cascade  "); got != "wiki/entities/Cascade.md" {
		t.Errorf("trimmed title: got %q", got)
	}
}

func TestResolveByAlias(t *testing.T) {
	r := NewResolver([]PageRef{
		{
			Path:    "wiki/entities/Cascade.md",
			Title:   "Cascade",
			Aliases: []string{"Claude Code for BigQuery", "CC4BQ"},
		},
	})
	if got := r.Resolve("CC4BQ"); got != "wiki/entities/Cascade.md" {
		t.Errorf("alias match: got %q", got)
	}
	if got := r.Resolve("claude code for bigquery"); got != "wiki/entities/Cascade.md" {
		t.Errorf("case-insensitive alias: got %q", got)
	}
}

func TestResolveBrokenReturnsEmpty(t *testing.T) {
	r := NewResolver(nil)
	if got := r.Resolve("Nonexistent"); got != "" {
		t.Errorf("broken link: got %q, want empty", got)
	}
}

func TestResolveOrTargetReturnsTargetWhenBroken(t *testing.T) {
	r := NewResolver(nil)
	if got := r.ResolveOrTarget("  Ghost  "); got != "Ghost" {
		t.Errorf("ResolveOrTarget: got %q, want 'Ghost'", got)
	}
}

func TestResolverFirstWinsOnDuplicateTitle(t *testing.T) {
	r := NewResolver([]PageRef{
		{Path: "wiki/entities/First.md", Title: "Dup"},
		{Path: "wiki/entities/Second.md", Title: "Dup"},
	})
	got := r.Resolve("Dup")
	if !strings.HasSuffix(got, "First.md") {
		t.Errorf("expected first-inserted to win, got %q", got)
	}
}
