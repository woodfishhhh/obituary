package contextmenu

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use: "context-menu <resurrect|policy> <path>...", Hidden: true, Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "policy":
				return proposePolicies(cmd, args[1:])
			case "resurrect":
				return resurrectLatest(cmd)
			default:
				return fmt.Errorf(cmdutil.Text("cli.context_menu.unknown_context_menu_action"), args[0])
			}
		},
	}
}

func proposePolicies(cmd *cobra.Command, paths []string) error {
	svc, err := cmdutil.Open()
	if err != nil {
		return err
	}
	svc = svc.WithContext(cmd.Context())
	for _, path := range paths {
		p, err := svc.SetWatch(path, "30d", ledger.WatchFirstSeen)
		if err != nil {
			return err
		}
		st, err := svc.Store.Read()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), cmdutil.Text("cli.context_menu.folder_policy"), p.ID, p.Path, p.Lifespan, p.Age, p.Consent)
		for _, proposal := range st.Proposals {
			if proposal.WatchID == p.ID {
				fmt.Fprintf(cmd.OutOrStdout(), cmdutil.Text("cli.watch.proposed"), proposal.ID)
			}
		}
	}
	return nil
}

func resurrectLatest(cmd *cobra.Command) error {
	svc, err := cmdutil.Open()
	if err != nil {
		return err
	}
	svc = svc.WithContext(cmd.Context())
	view, err := svc.ReadGraveyard()
	if err := cmdutil.Committed(err); err != nil {
		return err
	}
	ref, restored, err := latest(view)
	if err != nil {
		return err
	}
	if restored != nil {
		return cmdutil.ReportGraves([]lifecycle.GraveResult{*restored}, false, restoredText)
	}
	rows, err := svc.Resurrect([]string{ref})
	reportErr := cmdutil.ReportGraves(rows, false, restoredText)
	return errors.Join(cmdutil.Committed(err), reportErr)
}

func restoredText(r lifecycle.GraveResult) string {
	return fmt.Sprintf(cmdutil.Text("cli.context_menu.resurrected"), r.Burial.ID, r.Path)
}

func latest(view *lifecycle.GraveyardView) (string, *lifecycle.GraveResult, error) {
	// A scan/recovery problem must not silently select an older deletion.
	for _, pending := range view.Pending {
		if pending.Burial == nil || !slices.ContainsFunc(view.Restored, func(r lifecycle.GraveResult) bool { return r.Burial.ID == pending.Burial.ID }) {
			return "", nil, errors.New(cmdutil.Text("cli.context_menu.recovery_needed"))
		}
	}
	var best *ledger.Burial
	var receipt *lifecycle.GraveResult
	for _, b := range view.Burials {
		if newer(b, best) {
			copy := b
			best, receipt = &copy, nil
		}
	}
	for _, r := range view.Restored {
		if newer(*r.Burial, best) {
			copy := r
			best, receipt = r.Burial, &copy
		}
	}
	if best == nil {
		return "", nil, errors.New(cmdutil.Text("cli.context_menu.empty"))
	}
	return strconv.FormatInt(best.ID, 10), receipt, nil
}

func newer(b ledger.Burial, best *ledger.Burial) bool {
	return best == nil || b.BuriedAt.After(best.BuriedAt) || b.BuriedAt.Equal(best.BuriedAt) && b.ID > best.ID
}
