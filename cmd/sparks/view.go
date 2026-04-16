package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/yogirk/sparks/internal/view"
)

func newViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Browse the vault in your web browser (local, read-only)",
		Long: `Starts a lightweight HTTP server that renders the vault's wiki pages
as HTML. Read-only: edits happen in your editor or via your agent.
Defaults to 127.0.0.1:3030; pass --port or --addr to change.

Ctrl-C stops the server. File changes are picked up on the next page
refresh — no watcher, no cache.`,
		Args: cobra.NoArgs,
		RunE: runView,
	}
	cmd.Flags().String("addr", view.DefaultAddr, "address to listen on (host:port)")
	cmd.Flags().Bool("open", false, "open the default browser when the server boots")
	return cmd
}

func runView(cmd *cobra.Command, args []string) error {
	v, db, err := openVault(".")
	if err != nil {
		return err
	}
	defer db.Close()

	addr, _ := cmd.Flags().GetString("addr")
	openFlag, _ := cmd.Flags().GetBool("open")
	srv, err := view.NewServer(v, db, view.Options{Addr: addr, OpenOnBoot: openFlag})
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return srv.Run(ctx)
}
