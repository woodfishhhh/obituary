package cancel

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "cancel <id|path>...",
		Aliases: []string{"spare"},
		Short:   "Spare paths or container IDs: stop tracking them and keep the objects",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			results, err := svc.Cancel(args)
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			if asJSON {
				return cmdutil.Report(results, true)
			}
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintln(os.Stderr, "error:", r.Error)
				} else {
					if r.Item.Kind == core.KindContainer {
						fmt.Printf("#%d canceled %s (container kept)\n", r.Item.ID, r.Item.Path)
					} else {
						fmt.Printf("#%d canceled %s (kept on disk)\n", r.Item.ID, r.Item.Path)
					}
				}
			}
			if core.Failed(results) {
				return cmdutil.ErrReported
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
