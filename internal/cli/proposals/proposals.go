package proposals

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
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
				return cmdutil.PrintJSON(app.ProposalViews(svc, state.Proposals))
			}
			printProposals(lifecycle.ExistingProposals(state.Proposals))
			printHandled(state.AutoHandled(), time.Now())
			printMode(svc.Proposals())
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func printProposals(proposals []ledger.Proposal) {
	if len(proposals) == 0 {
		fmt.Println(cmdutil.Text("cli.proposals.no_pending_proposals"))
		return
	}
	for _, p := range proposals {
		fmt.Printf("#%d %s  %s  %s, %s\n", p.ID, cmdutil.Localizer().Label(p.Kind), p.Project, cmdutil.Localizer().Count("cli.count.item", len(p.Candidates)), textutil.Bytes(p.Size()))
		for _, c := range p.Candidates {
			rule := ""
			if c.Reason != "" {
				rule = "  " + c.Reason
			}
			if c.Rule != "" {
				rule = cmdutil.Text("cli.proposals.by") + c.Rule
			}
			fmt.Printf("    %-5s %9s  %s%s\n", c.Lifespan, textutil.Bytes(c.Size), c.Path, rule)
		}
	}
	fmt.Println(cmdutil.Text("cli.proposals.obit_approve_id_approves_the_displayed_action"))
}

// printHandled lists what the proposal mode let in without asking, while it
// is still tracked, with the commands that undo it.
func printHandled(handled []ledger.AutoHandled, now time.Time) {
	if len(handled) == 0 {
		return
	}
	fmt.Println(cmdutil.Text("cli.proposals.registered_without_asking"))
	for _, h := range handled {
		l := cmdutil.Localizer()
		fmt.Printf(l.Text("cli.proposals.expires_printhandled", nil), h.Item.ID, cmdutil.Column(l, h.Class), cmdutil.Column(l, cmdutil.DueIn(h.Item.ExpiresAt.Sub(now))), h.Item.Path)
	}
	fmt.Println(cmdutil.Text("cli.proposals.obit_spare_id_undoes_one_the_file"))
}

var modeMessages = map[string]string{
	lifecycle.ProposalsOff:    "cli.proposals.off_nothing_goes_in_by_itself_agents",
	lifecycle.ProposalsTiered: "cli.proposals.tiered_files_the_agent_created_in_its",
	lifecycle.ProposalsAuto:   "cli.proposals.auto_everything_automatic_registration_finds_goes_in",
	lifecycle.ProposalsAsk:    "cli.proposals.ask_every_automatic_registration_waits_here_first",
}

func printMode(mode string) {
	path, err := config.Path()
	if err != nil {
		path = "config.toml"
	}
	fmt.Printf(cmdutil.Text("cli.proposals.proposal_mode"), cmdutil.Text(modeMessages[mode]))
	switch mode {
	case lifecycle.ProposalsOff:
		fmt.Printf(cmdutil.Text("cli.proposals.to_turn_automatic_registration_on_set_proposals"), path)
	case lifecycle.ProposalsAuto:
		fmt.Printf(cmdutil.Text("cli.proposals.to_be_asked_again_set_proposals_mode"), path)
	default:
		fmt.Printf(cmdutil.Text("cli.proposals.to_let_agents_artefacts_in_without_asking"), path)
	}
}
