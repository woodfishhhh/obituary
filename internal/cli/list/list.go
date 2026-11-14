package list

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifespan"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show tracked paths and Docker containers, soonest expiry first",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			state, err := svc.Store.Read()
			if err != nil {
				return err
			}
			now := time.Now()
			if asJSON {
				return printItemsJSON(svc, state.Items, now)
			}
			return printItems(svc, state.Items, now)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func printItemsJSON(svc *core.Service, items []ledger.Item, now time.Time) error {
	type row struct {
		ledger.Item
		Stability       *core.WatchStatus `json:"stability,omitempty"`
		IdentityWarning string            `json:"identityWarning,omitempty"`
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		rows = append(rows, row{it, svc.WatchStatus(it, now), identityWarning(it)})
	}
	return cmdutil.PrintJSON(rows)
}

func printItems(svc *core.Service, items []ledger.Item, now time.Time) error {
	if len(items) == 0 {
		fmt.Println("nothing is tracked")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tEXPIRES\tLEFT\tPATH\tNOTE")
	for _, item := range items {
		note := item.Note
		if item.Stuck() {
			// Extend clears the failure count, so it is the command-line Retry.
			note = fmt.Sprintf("stopped retrying after %d attempts: %s; obit extend #%d 1m tries again, obit spare #%d stops tracking", item.Attempts, item.LastError, item.ID, item.ID)
		} else if item.LastError != "" {
			note = fmt.Sprintf("retry %d: %s", item.Attempts, item.LastError)
		}
		if warning := identityWarning(item); warning != "" {
			note += "; " + warning
		}
		if status := svc.WatchStatus(item, now); status != nil {
			if strings.HasPrefix(item.LastError, "folder child is still active") {
				note = fmt.Sprintf("retry %d", item.Attempts)
			}
			note += fmt.Sprintf("; quiet for %s more (stable_window=%s); next check %s", status.QuietFor, status.StableWindow, status.NextCheck.Format(time.RFC3339))
		}
		fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s\n", item.ID, item.ExpiresAt.Local().Format(cmdutil.TimeLayout),
			lifespan.FormatRemaining(time.Until(item.ExpiresAt)), item.Path, note)
	}
	return w.Flush()
}

func identityWarning(item ledger.Item) string {
	if item.Identity != nil {
		if err := item.Identity.RequirePersistent(); err != nil {
			return err.Error()
		}
	}
	return ""
}
