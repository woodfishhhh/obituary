package graveyard

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifespan"
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
		fmt.Printf("#%d resurrected  %s (recovered)\n", r.Burial.ID, r.Path)
	}
	if len(listing.Burials) == 0 && len(listing.Pending) == 0 && scanErr == nil {
		fmt.Println("the graveyard is empty")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tBURIED\tPURGE\tSIZE\tPATH")
	for _, b := range listing.Burials {
		printBurial(w, b)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Printf("pending recovery: %d\n", len(listing.Pending))
	for _, p := range listing.Pending {
		fmt.Printf("%s  %s", p.Status, p.Manifest)
		if p.Error != "" {
			fmt.Printf("  WARN: %s", p.Error)
		}
		fmt.Println()
	}
	if scanErr != nil {
		fmt.Fprintln(os.Stderr, "warning:", scanErr)
	}
	return nil
}

func printBurial(w io.Writer, b ledger.Burial) {
	purge := "in " + lifespan.FormatRemaining(time.Until(b.PurgeAt))
	if b.PurgeIntent != nil {
		retry := b.NextPurgeAttempt
		if retry.IsZero() {
			retry = b.PurgeIntent.VerifiedAt
		}
		purge = "purging (retry at " + retry.Local().Format(cmdutil.TimeLayout) + ")"
	}
	if b.IdentityMismatch != "" {
		purge = "never (identity mismatch)"
	}
	fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s", b.ID, b.BuriedAt.Local().Format(cmdutil.TimeLayout),
		purge, cmdutil.HumanBytes(b.Size), b.Path)
	if len(b.Contains) > 0 {
		fmt.Fprint(w, "  contains")
		for _, id := range b.Contains {
			fmt.Fprintf(w, " #%d", id)
		}
	}
	if b.PurgeIntent != nil && b.LastPurgeError != "" {
		fmt.Fprint(w, "  WARN: ", b.LastPurgeError)
	}
	if b.IdentityMismatch != "" {
		fmt.Fprint(w, "  WARN: ", b.IdentityMismatch)
	}
	fmt.Fprintln(w)
}
