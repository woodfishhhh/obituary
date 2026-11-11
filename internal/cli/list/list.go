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
				type row struct {
					core.Item
					Stability *core.WatchStatus `json:"stability,omitempty"`
				}
				rows := make([]row, 0, len(state.Items))
				for _, it := range state.Items {
					rows = append(rows, row{it, svc.WatchStatus(it, now)})
				}
				return cmdutil.PrintJSON(rows)
			}
			if len(state.Items) == 0 {
				fmt.Println("nothing is tracked")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tEXPIRES\tLEFT\tPATH\tNOTE")
			for _, item := range state.Items {
				note := item.Note
				if item.LastError != "" {
					note = fmt.Sprintf("retry %d: %s", item.Attempts, item.LastError)
				}
				if status := svc.WatchStatus(item, now); status != nil {
					if strings.HasPrefix(item.LastError, "folder child is still active") {
						note = fmt.Sprintf("retry %d", item.Attempts)
					}
					note += fmt.Sprintf("; quiet for %s more (stable_window=%s); next check %s", status.QuietFor, status.StableWindow, status.NextCheck.Format(time.RFC3339))
				}
				fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s\n", item.ID, item.ExpiresAt.Local().Format(cmdutil.TimeLayout),
					core.FormatRemaining(time.Until(item.ExpiresAt)), item.Path, note)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
