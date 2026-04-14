package contract

import (
	"strings"
	"testing"
)

func TestMarkdownIsEmbedded(t *testing.T) {
	body := Markdown()
	if len(body) == 0 {
		t.Fatal("contract markdown is empty — //go:embed didn't pick up contract.md")
	}
	// Anchor checks: known phrases that must survive any future contract edits.
	anchors := []string{
		"Sparks",
		"raw/",
		"wiki/",
		"sparks ingest",
		"AGENTS.md",
	}
	for _, a := range anchors {
		if !strings.Contains(body, a) {
			t.Errorf("contract missing anchor phrase %q — has the doc drifted?", a)
		}
	}
}

func TestKnownAgents(t *testing.T) {
	for _, name := range KnownAgents {
		if !IsKnown(name) {
			t.Errorf("KnownAgents lists %q but IsKnown reports false", name)
		}
		if Filename(name) == "" {
			t.Errorf("agent %q has no filename mapping", name)
		}
	}
}

func TestUnknownAgent(t *testing.T) {
	if IsKnown("notathing") {
		t.Error("unknown agent should not be IsKnown")
	}
	if Filename("notathing") != "" {
		t.Error("unknown agent should have empty filename")
	}
}

func TestClaudeAndGeminiHaveDistinctFilenames(t *testing.T) {
	if Filename(AgentClaude) == Filename(AgentGemini) {
		t.Error("claude and gemini must use distinct instruction filenames")
	}
}
