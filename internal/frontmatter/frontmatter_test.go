package frontmatter

import (
	"errors"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	in := `---
title: Cascade
type: entity
maturity: working
tags: [data-engineering, gcp]
aliases: [Claude Code for BigQuery]
sources: [raw/inbox/2026-04-01.md]
created: 2026-04-01
updated: 2026-04-13
---

Body here.
`
	fm, body, err := ParseBytes([]byte(in))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if fm.Title != "Cascade" || fm.Type != "entity" || fm.Maturity != "working" {
		t.Errorf("fm = %+v", fm)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "data-engineering" {
		t.Errorf("tags = %v", fm.Tags)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "Body here") {
		t.Errorf("body = %q", string(body))
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	_, _, err := ParseBytes([]byte("# Plain markdown\nNo frontmatter."))
	if !errors.Is(err, ErrNoFrontmatter) {
		t.Errorf("err = %v, want ErrNoFrontmatter", err)
	}
}

func TestParseUnclosedDelimiter(t *testing.T) {
	_, _, err := ParseBytes([]byte("---\ntitle: oops\n"))
	if !errors.Is(err, ErrUnclosed) {
		t.Errorf("err = %v, want ErrUnclosed", err)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name      string
		fm        Frontmatter
		wantIssue string
	}{
		{"missing title", Frontmatter{Type: "entity", Maturity: "seed", Created: "x", Updated: "x", Sources: []string{"a"}}, "title"},
		{"bad type", Frontmatter{Title: "x", Type: "bogus", Maturity: "seed", Created: "x", Updated: "x", Sources: []string{"a"}}, "invalid type"},
		{"bad maturity", Frontmatter{Title: "x", Type: "entity", Maturity: "rotten", Created: "x", Updated: "x", Sources: []string{"a"}}, "invalid maturity"},
		{"missing sources", Frontmatter{Title: "x", Type: "entity", Maturity: "seed", Created: "x", Updated: "x"}, "sources"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := Validate(c.fm)
			if len(issues) == 0 {
				t.Fatal("expected at least one issue")
			}
			joined := strings.Join(issues, "|")
			if !strings.Contains(joined, c.wantIssue) {
				t.Errorf("issues %v missing %q", issues, c.wantIssue)
			}
		})
	}
}

func TestValidateClean(t *testing.T) {
	fm := Frontmatter{
		Title: "x", Type: "entity", Maturity: "seed",
		Created: "2026-01-01", Updated: "2026-01-01",
		Sources: []string{"raw/inbox/2026-01-01.md"},
	}
	if issues := Validate(fm); len(issues) != 0 {
		t.Errorf("Validate clean fm returned issues: %v", issues)
	}
}
