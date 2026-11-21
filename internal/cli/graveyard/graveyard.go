package graveyard

import (
	"fmt"
	"io"
	"os"

	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifespan"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

type GraveyardListing struct {
	Burials      []ledger.Burial     `json:"burials"`
	Pending      []core.PendingGrave `json:"pending"`
	PendingCount int                 `json:"pendingCount"`
	Restored     []core.GraveResult  `json:"restored,omitempty"`
	Warning      string              `json:"warning,omitempty"`
}

func NewCmd() *cobra.Command {
	var asJSON, here bool
	cmd := &cobra.Command{
		Use:     "graveyard",
		Aliases: []string{"graves"},
		Short:   "List what lies in the graveyard and can still be resurrected",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			view, scanErr := svc.WithContext(cmd.Context()).ReadGraveyard()
			if view == nil {
				return scanErr
			}
			if here {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				if err := svc.FilterGraveyard(view, cwd); err != nil {
					return err
				}
			}
			listing := GraveyardListing{Burials: view.Burials, Pending: view.Pending, PendingCount: len(view.Pending), Restored: view.Restored}
			if scanErr != nil {
				listing.Warning = scanErr.Error()
			}
			if asJSON {
				return cmdutil.PrintJSON(listing)
			}
			return printListing(listing, scanErr)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&here, "here", false, "show only original paths in the current directory and its descendants")
	return cmd
}

func printListing(listing GraveyardListing, scanErr error) error {
	for _, r := range listing.Restored {
		fmt.Printf(cmdutil.Text("cli.graveyard.resurrected_recovered"), r.Burial.ID, r.Path)
	}
	if len(listing.Burials) == 0 && len(listing.Pending) == 0 && scanErr == nil {
		fmt.Println(cmdutil.Text("cli.graveyard.empty"))
		return nil
	}
	w := cmdutil.Table(os.Stdout, cmdutil.Language())
	fmt.Fprintln(w, cmdutil.Text("cli.graveyard.header"))
	for _, b := range listing.Burials {
		printBurial(w, b)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Printf(cmdutil.Text("cli.daemon.pending_recovery"), len(listing.Pending))
	for _, p := range listing.Pending {
		fmt.Printf("%s  %s", cmdutil.Localizer().Label(p.Status), p.Manifest)
		if p.Error != "" {
			fmt.Printf(cmdutil.Text("cli.graveyard.warn"), p.Error)
		}
		fmt.Println()
	}
	if scanErr != nil {
		fmt.Fprintln(os.Stderr, cmdutil.Text("cli.warning_prefix"), scanErr)
	}
	return nil
}

func printBurial(w io.Writer, b ledger.Burial) {
	purge := cmdutil.Text("cli.time.in_prefix") + lifespan.FormatRemaining(time.Until(b.PurgeAt))
	if b.PurgeIntent != nil {
		retry := b.NextPurgeAttempt
		if retry.IsZero() {
			retry = b.PurgeIntent.VerifiedAt
		}
		purge = cmdutil.Text("cli.graveyard.purging_retry_at") + retry.Local().Format(cmdutil.TimeLayout) + ")"
	}
	if b.IdentityMismatch != "" {
		purge = cmdutil.Text("cli.graveyard.never_identity_mismatch")
	}
	fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s", b.ID, b.BuriedAt.Local().Format(cmdutil.TimeLayout),
		purge, textutil.Bytes(b.Size), b.Path)
	if len(b.Contains) > 0 {
		fmt.Fprint(w, cmdutil.Text("cli.graveyard.contains"))
		for _, id := range b.Contains {
			fmt.Fprintf(w, " #%d", id)
		}
	}
	if b.PurgeIntent != nil && b.LastPurgeError != "" {
		fmt.Fprint(w, cmdutil.Text("cli.graveyard.warn_printburial"), b.LastPurgeError)
	}
	if b.IdentityMismatch != "" {
		fmt.Fprint(w, cmdutil.Text("cli.graveyard.warn_printburial"), b.IdentityMismatch)
	}
	fmt.Fprintln(w)
}
