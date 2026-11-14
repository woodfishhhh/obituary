package proposals

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "proposals",
		Short: "Registrations waiting for your approval, and those made without asking",
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
			printProposals(core.ExistingProposals(state.Proposals))
			printHandled(state.AutoHandled(), time.Now())
			printMode(svc.Proposals())
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func printProposals(proposals []core.Proposal) {
	if len(proposals) == 0 {
		fmt.Println("no pending proposals")
		return
	}
	for _, p := range proposals {
		fmt.Printf("#%d %s  %s  %s, %s\n", p.ID, p.Kind, p.Project, textutil.Count(len(p.Candidates), "item"), textutil.Bytes(p.Size()))
		for _, c := range p.Candidates {
			rule := ""
			if c.Reason != "" {
				rule = "  " + c.Reason
			}
			if c.Rule != "" {
				rule = "  by " + c.Rule
			}
			fmt.Printf("    %-5s %9s  %s%s\n", c.Lifespan, textutil.Bytes(c.Size), c.Path, rule)
		}
	}
	fmt.Println("\nobit approve <id> approves the displayed action (expired folders are buried); obit reject <id> declines it (expired folders are spared).")
}

// printHandled lists what the proposal mode let in without asking, while it
// is still tracked, with the commands that undo it.
func printHandled(handled []core.AutoHandled, now time.Time) {
	if len(handled) == 0 {
		return
	}
	fmt.Println("\nregistered without asking:")
	for _, h := range handled {
		fmt.Printf("    #%-5d %-11s expires %-10s %s\n", h.Item.ID, h.Class, textutil.DueIn(h.Item.ExpiresAt.Sub(now)), h.Item.Path)
	}
	fmt.Println("obit spare <id> undoes one (the file stays, without a lifespan); obit extend <id> <lifespan> keeps it longer; once buried, obit resurrect <id> brings it back.")
}

var modeText = map[string]string{
	core.ProposalsTiered: "tiered: files the agent created in its session, regenerable leftovers and session containers go in without asking; the rest waits here",
	core.ProposalsAuto:   "auto: everything automatic registration finds goes in without asking",
	core.ProposalsAsk:    "ask: every automatic registration waits here first",
}

func printMode(mode string) {
	path, err := config.Path()
	if err != nil {
		path = "config.toml"
	}
	fmt.Printf("\nproposal mode %s.\n", modeText[mode])
	switch mode {
	case core.ProposalsAuto:
		fmt.Printf("To be asked again, set [proposals] mode = \"tiered\" (or \"ask\") in %s, or switch it on the window's Decisions page.\n", path)
	default:
		fmt.Printf("To let agents' artefacts in without asking, set [proposals] mode = \"auto\" in %s, or switch it on the window's Decisions page.\n", path)
	}
}
