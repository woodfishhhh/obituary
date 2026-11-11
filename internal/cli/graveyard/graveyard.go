package graveyard

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

type GraveyardListing struct {
	Burials      []core.Burial       `json:"burials"`
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
			pending := view.Pending
			listing := GraveyardListing{Burials: view.Burials, Pending: pending, PendingCount: len(pending), Restored: view.Restored}
			if scanErr != nil {
				listing.Warning = scanErr.Error()
			}
			if asJSON {
				return cmdutil.PrintJSON(listing)
			}
			for _, r := range view.Restored {
				fmt.Printf("#%d resurrected  %s (recovered)\n", r.Burial.ID, r.Path)
			}
			if len(view.Burials) == 0 && len(pending) == 0 && scanErr == nil {
				fmt.Println("the graveyard is empty")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tBURIED\tPURGE\tSIZE\tPATH")
			for _, b := range view.Burials {
				purge := "in " + core.FormatRemaining(time.Until(b.PurgeAt))
				if b.PurgeIntent != nil {
					retry := b.NextPurgeAttempt
					if retry.IsZero() {
						retry = b.PurgeIntent.VerifiedAt
					}
					purge = "purging (retry at " + retry.Local().Format(cmdutil.TimeLayout) + ")"
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
				fmt.Fprintln(w)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Printf("pending recovery: %d\n", len(pending))
			for _, p := range pending {
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
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&here, "here", false, "show only original paths in the current directory and its descendants")
	return cmd
}
