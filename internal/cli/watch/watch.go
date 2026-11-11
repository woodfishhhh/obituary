package watch

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	folderwatch "github.com/woodfishhhh/obituary/internal/watch"
)

type options struct {
	list, asJSON, noDaemon             bool
	remove, age                        string
	removeID, policyID, clearExclusion int64
}

func NewCmd() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "watch <directory> <lifespan> | watch --list | watch --remove <directory>",
		Short: "Give each top-level child of a folder its own lifespan",
		Args: func(cmd *cobra.Command, args []string) error {
			if o.list || o.remove != "" || o.removeID != 0 || o.clearExclusion != 0 {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error { return o.run(args) },
	}
	cmd.Flags().BoolVar(&o.list, "list", false, "list folder policies")
	cmd.Flags().StringVar(&o.remove, "remove", "", "remove a policy and cancel its registered lifespans")
	cmd.Flags().StringVar(&o.age, "age", core.WatchFirstSeen, "age basis: first-seen or modified (requires approval)")
	cmd.Flags().BoolVar(&o.asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&o.noDaemon, "no-daemon", false, "do not start the background runner")
	cmd.Flags().Int64Var(&o.removeID, "remove-id", 0, "remove a folder policy by ID, including missing paths")
	cmd.Flags().Int64Var(&o.policyID, "policy", 0, "policy ID whose exclusion should be cleared")
	cmd.Flags().Int64Var(&o.clearExclusion, "clear-exclusion", 0, "explicitly clear this exclusion ID (with --policy)")
	cmd.MarkFlagsMutuallyExclusive("list", "remove", "remove-id", "clear-exclusion", "age")
	cmd.MarkFlagsRequiredTogether("policy", "clear-exclusion")
	return cmd
}

func (o *options) run(args []string) error {
	svc, err := cmdutil.Open()
	if err != nil {
		return err
	}
	if o.list {
		return o.printPolicies(svc)
	}
	if o.removeID != 0 || o.clearExclusion != 0 {
		return o.changePolicy(svc)
	}
	if o.remove != "" {
		path, err := expandHome(o.remove)
		if err != nil {
			return err
		}
		if err := svc.RemoveWatch(path); err != nil {
			return err
		}
		if o.asJSON {
			return cmdutil.PrintJSON(map[string]string{"removed": path})
		}
		fmt.Println("folder policy removed:", path)
		return nil
	}
	path, err := expandHome(args[0])
	if err != nil {
		return err
	}
	p, err := svc.SetWatch(path, args[1], o.age)
	if err != nil {
		return err
	}
	if !o.noDaemon {
		cmdutil.EnsureDaemon(svc.Store.Home, o.asJSON)
	}
	if o.asJSON {
		err = cmdutil.PrintJSON(p)
	} else {
		fmt.Printf("#%d %s  %s  age=%s  %s\n", p.ID, p.Path, p.Lifespan, p.Age, p.Consent)
		if p.Consent != core.ConsentApproved {
			if err := printWatchProposal(svc, p.ID); err != nil {
				return err
			}
		}
	}
	if err != nil {
		return err
	}
	return folderwatch.Reconcile(svc)
}

func printWatchProposal(svc *core.Service, policyID int64) error {
	st, err := svc.Store.Read()
	if err != nil {
		return err
	}
	for _, p := range st.Proposals {
		if p.WatchID == policyID {
			fmt.Printf("已提议，用 obit approve %d 批准。\n", p.ID)
			break
		}
	}
	return nil
}

func (o *options) changePolicy(svc *core.Service) error {
	if o.removeID != 0 {
		if err := svc.RemoveWatchID(o.removeID); err != nil {
			return err
		}
		if o.asJSON {
			return cmdutil.PrintJSON(map[string]int64{"removedId": o.removeID})
		}
		fmt.Printf("folder policy removed: #%d\n", o.removeID)
		return nil
	}
	if err := svc.ClearWatchExclusion(o.policyID, o.clearExclusion); err != nil {
		return err
	}
	if o.asJSON {
		return cmdutil.PrintJSON(map[string]int64{"policyId": o.policyID, "clearedExclusion": o.clearExclusion})
	}
	fmt.Printf("folder policy #%d: exclusion #%d cleared\n", o.policyID, o.clearExclusion)
	return nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}
	home, err := userdirs.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimLeft(path[1:], `/\`)), nil
}
