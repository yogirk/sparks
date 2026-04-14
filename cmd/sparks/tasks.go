package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/core"
)

func newTasksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Manage the live Tasks collection",
	}
	cmd.AddCommand(newTasksAddCmd())
	return cmd
}

func newTasksAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Append a task under a section heading in wiki/collections/Tasks.md",
		Args:  cobra.NoArgs,
		RunE:  runTasksAdd,
	}
	cmd.Flags().String("section", "", "section heading (e.g. \"[[Sparks]]\"); created if missing")
	cmd.Flags().String("text", "", "task text (added as `- [ ] {text}`)")
	_ = cmd.MarkFlagRequired("section")
	_ = cmd.MarkFlagRequired("text")
	return cmd
}

func runTasksAdd(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()
	section, _ := cmd.Flags().GetString("section")
	text, _ := cmd.Flags().GetString("text")
	res, err := core.TasksAdd(v, section, text)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if res.SectionCreate {
		fmt.Fprintf(out, "Created section %q.\n", res.Section)
	}
	fmt.Fprintf(out, "Added: %s (line %d under %s)\n", res.Text, res.Line, res.Section)
	return nil
}
