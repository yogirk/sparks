package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yogirk/sparks/internal/contract"
)

// ErrUnknownAgent is returned by WriteAgentFile when the agent name
// isn't one of the recognized harnesses.
var ErrUnknownAgent = errors.New("unknown agent (want claude|codex|gemini|generic)")

// WriteAgentFile writes the canonical contract to the conventional
// instruction filename for the given agent under vaultRoot.
//
// Returns (wrote, filename, err):
//
//	wrote=true  → file was created
//	wrote=false → file already existed; pass force=true to overwrite
//
// The contract content is identical across agents — only the filename
// differs (CLAUDE.md vs AGENTS.md vs GEMINI.md). This is the central
// claim of Approach B from the design doc: the binary owns the
// protocol; per-agent files are just bridges.
func WriteAgentFile(vaultRoot string, agent contract.AgentName, force bool) (bool, string, error) {
	if !contract.IsKnown(agent) {
		return false, "", fmt.Errorf("%w: %q", ErrUnknownAgent, agent)
	}
	filename := contract.Filename(agent)
	target := filepath.Join(vaultRoot, filename)
	if !force {
		if _, err := os.Stat(target); err == nil {
			return false, filename, nil
		}
	}
	if err := os.WriteFile(target, []byte(contract.Markdown()), 0o644); err != nil {
		return false, filename, fmt.Errorf("write %s: %w", target, err)
	}
	return true, filename, nil
}
