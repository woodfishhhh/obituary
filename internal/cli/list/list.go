package list

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/daemonipc"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/listing"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// residentTimeout is how long the command waits for a resident before it reads
// the ledger itself. An answer normally takes a few milliseconds.
const residentTimeout = 2 * time.Second

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show tracked paths and Docker containers, soonest expiry first",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if printed, err := printedByResident(asJSON); printed {
				return err
			}
			return printLocally(asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmdutil.SetLocalizedHelp(cmd, func(language *i18n.Localizer, command *cobra.Command) {
		command.Short = language.Text("cli.list.short", nil)
		command.Flags().Lookup("json").Usage = language.Text("cli.list.json", nil)
	})
	return cmd
}

// printedByResident has a running resident print the list from the ledger it
// already holds, which spares this process opening and decoding the ledger. It
// reports false, having written nothing, whenever there is no resident or it
// does not answer; the caller then reads the ledger itself.
func printedByResident(asJSON bool) (bool, error) {
	cfg, err := config.Load()
	if err != nil {
		return false, nil // the local path reports a bad configuration
	}
	home, err := userdirs.Data()
	if err != nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), residentTimeout)
	defer cancel()
	out, err := daemonipc.List(ctx, home, daemonipc.ListRequest{
		Language:     string(i18n.Resolve(cfg.UI.Language, i18n.SystemLanguage)),
		JSON:         asJSON,
		StableWindow: cfg.Watch.Stability(),
	})
	if err != nil {
		return false, nil
	}
	_, err = os.Stdout.Write(out)
	return true, err
}

func printLocally(asJSON bool) error {
	svc, err := cmdutil.Open()
	if err != nil {
		return err
	}
	state, err := svc.Store.ReadActive()
	if err != nil {
		return err
	}
	return listing.Write(os.Stdout, state.Items, time.Now(), listing.Options{
		Language: cmdutil.Language(), JSON: asJSON, StableWindow: svc.WatchStableWindow,
	})
}
