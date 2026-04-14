package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yogirk/sparks/internal/contract"
)

func TestWriteAgentFileFreshClaude(t *testing.T) {
	dir := t.TempDir()
	wrote, name, err := WriteAgentFile(dir, contract.AgentClaude, false)
	if err != nil {
		t.Fatalf("WriteAgentFile: %v", err)
	}
	if !wrote {
		t.Error("wrote = false on fresh dir")
	}
	if name != "CLAUDE.md" {
		t.Errorf("name = %q, want CLAUDE.md", name)
	}
	body := readFile(t, filepath.Join(dir, name))
	if !strings.Contains(body, "Sparks") {
		t.Error("CLAUDE.md missing contract content")
	}
}

func TestWriteAgentFileIdempotentSkip(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := WriteAgentFile(dir, contract.AgentClaude, false); err != nil {
		t.Fatal(err)
	}
	wrote, _, err := WriteAgentFile(dir, contract.AgentClaude, false)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Error("second call without --force should skip, got wrote=true")
	}
}

func TestWriteAgentFileForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(target, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrote, _, err := WriteAgentFile(dir, contract.AgentClaude, true)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Error("--force should overwrite, got wrote=false")
	}
	body := readFile(t, target)
	if strings.Contains(body, "old content") {
		t.Error("--force did not replace prior content")
	}
}

func TestWriteAgentFileUnknownAgent(t *testing.T) {
	dir := t.TempDir()
	_, _, err := WriteAgentFile(dir, "definitely-not-a-harness", false)
	if !errors.Is(err, ErrUnknownAgent) {
		t.Errorf("err = %v, want ErrUnknownAgent", err)
	}
}

func TestWriteAgentFileEachKnownAgent(t *testing.T) {
	for _, agent := range contract.KnownAgents {
		t.Run(string(agent), func(t *testing.T) {
			dir := t.TempDir()
			wrote, name, err := WriteAgentFile(dir, agent, false)
			if err != nil {
				t.Fatal(err)
			}
			if !wrote {
				t.Error("fresh dir, expected wrote=true")
			}
			if name == "" {
				t.Error("no filename returned")
			}
		})
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
