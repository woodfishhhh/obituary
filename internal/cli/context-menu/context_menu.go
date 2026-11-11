package contextmenu

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
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
				return fmt.Errorf("unknown context menu action %q", args[0])
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
		p, err := svc.SetWatch(path, "30d", core.WatchFirstSeen)
		if err != nil {
			return err
		}
		st, err := svc.Store.Read()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "文件夹策略 #%d：%s，%s，age=%s，%s。\n", p.ID, p.Path, p.Lifespan, p.Age, p.Consent)
		for _, proposal := range st.Proposals {
			if proposal.WatchID == p.ID {
				fmt.Fprintf(cmd.OutOrStdout(), "已提议，用 obit approve %d 批准。\n", proposal.ID)
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
		return cmdutil.ReportGraves([]core.GraveResult{*restored}, false, restoredText)
	}
	rows, err := svc.Resurrect([]string{ref})
	reportErr := cmdutil.ReportGraves(rows, false, restoredText)
	return errors.Join(cmdutil.Committed(err), reportErr)
}

func restoredText(r core.GraveResult) string {
	return fmt.Sprintf("已复活 #%d  %s", r.Burial.ID, r.Path)
}

func latest(view *core.GraveyardView) (string, *core.GraveResult, error) {
	// A scan/recovery problem must not silently select an older deletion.
	for _, pending := range view.Pending {
		if pending.Burial == nil || !slices.ContainsFunc(view.Restored, func(r core.GraveResult) bool { return r.Burial.ID == pending.Burial.ID }) {
			return "", nil, errors.New("墓地仍有待恢复记录，请先运行 obit graveyard 和 obit reap")
		}
	}
	var best *core.Burial
	var receipt *core.GraveResult
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
		return "", nil, errors.New("墓地为空；回收站模式下的文件请从系统回收站还原")
	}
	return strconv.FormatInt(best.ID, 10), receipt, nil
}

func newer(b core.Burial, best *core.Burial) bool {
	return best == nil || b.BuriedAt.After(best.BuriedAt) || b.BuriedAt.Equal(best.BuriedAt) && b.ID > best.ID
}
