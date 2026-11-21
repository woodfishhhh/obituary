package list

import (
	"fmt"
	"os"
	"strings"

	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifespan"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	language := cmdutil.Localizer()
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   language.Text("cli.list.short", nil),
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
			return printItems(svc, state.Items, now, cmdutil.Localizer())
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, language.Text("cli.list.json", nil))
	cmdutil.SetLocalizedHelp(cmd, func(language *i18n.Localizer, command *cobra.Command) {
		command.Short = language.Text("cli.list.short", nil)
		command.Flags().Lookup("json").Usage = language.Text("cli.list.json", nil)
	})
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

func printItems(svc *core.Service, items []ledger.Item, now time.Time, language *i18n.Localizer) error {
	if len(items) == 0 {
		fmt.Println(language.Text("cli.list.empty", nil))
		return nil
	}
	w := cmdutil.Table(os.Stdout, language.Language())
	fmt.Fprintln(w, language.Text("cli.list.header", nil))
	for _, item := range items {
		note := item.Note
		if item.Stuck() {
			// Extend clears the failure count, so it is the command-line Retry.
			note = language.Text("cli.list.stopped", map[string]any{"Attempts": item.Attempts, "Error": item.LastError, "ID": item.ID})
		} else if item.LastError != "" {
			note = language.Text("cli.list.retry", map[string]any{"Attempts": item.Attempts, "Error": item.LastError})
		}
		if warning := identityWarning(item); warning != "" {
			note += "; " + warning
		}
		if status := svc.WatchStatus(item, now); status != nil {
			if strings.HasPrefix(item.LastError, "folder child is still active") {
				note = fmt.Sprintf(language.Text("cli.list.retry_count", nil), item.Attempts)
			}
			note += "; " + language.Text("cli.list.quiet", map[string]any{"QuietFor": status.QuietFor, "StableWindow": status.StableWindow, "NextCheck": status.NextCheck.Format(time.RFC3339)})
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
