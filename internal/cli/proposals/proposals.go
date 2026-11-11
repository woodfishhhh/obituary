package proposals

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "proposals",
		Short: "Registrations waiting for your approval",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			state, err := svc.Store.Read()
			if err != nil {
				return err
			}
			if asJSON {
				return cmdutil.PrintJSON(core.ExistingProposals(state.Proposals))
			}
			state.Proposals = core.ExistingProposals(state.Proposals)
			if len(state.Proposals) == 0 {
				fmt.Println("no pending proposals")
				return nil
			}
			for _, p := range state.Proposals {
				fmt.Printf("#%d %s  %s  %s, %s\n", p.ID, p.Kind, p.Project, textutil.Count(len(p.Candidates), "item"), cmdutil.HumanBytes(p.Size()))
				for _, c := range p.Candidates {
					rule := ""
					if c.Reason != "" {
						rule = "  " + c.Reason
					}
					if c.Rule != "" {
						rule = "  by " + c.Rule
					}
					fmt.Printf("    %-5s %9s  %s%s\n", c.Lifespan, cmdutil.HumanBytes(c.Size), c.Path, rule)
				}
			}
			fmt.Println("\nobit approve <id> approves the displayed action (expired folders are buried); obit reject <id> declines it (expired folders are spared).")
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
