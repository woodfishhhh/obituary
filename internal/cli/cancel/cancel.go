package cancel

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
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
					fmt.Fprintln(os.Stderr, cmdutil.Text("cli.error_prefix"), cmdutil.ReferenceText(cmdutil.Localizer(), r.Err, r.Error))
				} else {
					if r.Item.Kind == ledger.KindContainer {
						fmt.Printf(cmdutil.Text("cli.cancel.canceled_container_kept"), r.Item.ID, r.Item.Path)
					} else {
						fmt.Printf(cmdutil.Text("cli.cancel.canceled_kept_on_disk"), r.Item.ID, r.Item.Path)
					}
				}
			}
			if lifecycle.Failed(results) {
				return cmdutil.ErrReported
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
