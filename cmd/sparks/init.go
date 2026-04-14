package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/contract"
	"github.com/yogirk/sparks/internal/core"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a Sparks vault (idempotent)",
		Long: `Initialize a Sparks vault. Creates sparks.toml, the canonical
directory layout, an empty inbox.md, and a fresh sparks.db. Idempotent:
running it on an existing vault repairs missing dirs without touching
content.

Pass --agent X to also drop the contract as a per-agent instruction
file (CLAUDE.md, AGENTS.md, GEMINI.md). The contract content is the
same across all agents — only the filename differs to match each
harness's convention.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runInit,
	}
	cmd.Flags().String("agent", "", "drop instruction file for harness: claude|codex|gemini|generic")
	cmd.Flags().Bool("force", false, "overwrite an existing instruction file when used with --agent")
	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	res, err := core.InitVault(path)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if res.Existed {
		fmt.Fprintf(out, "Vault already exists at %s — repaired any missing dirs/inbox.\n", res.VaultRoot)
	} else {
		fmt.Fprintf(out, "Initialized Sparks vault at %s\n", res.VaultRoot)
	}
	if res.ManifestNew {
		fmt.Fprintln(out, "  Created sparks.db (schema v1)")
	}

	agent, _ := cmd.Flags().GetString("agent")
	if agent == "" {
		return nil
	}
	force, _ := cmd.Flags().GetBool("force")
	wrote, filename, err := core.WriteAgentFile(res.VaultRoot, contract.AgentName(agent), force)
	if err != nil {
		return err
	}
	if wrote {
		fmt.Fprintf(out, "  Wrote %s for agent %q.\n", filename, agent)
	} else {
		fmt.Fprintf(out, "  %s already exists — pass --force to overwrite.\n", filename)
	}
	return nil
}
