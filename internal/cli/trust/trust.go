package trust

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	rulescmd "github.com/woodfishhhh/obituary/internal/cli/rules"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func NewCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "trust [rules file]",
		Short: "Review and trust the project's .obitrules (edits need a new review)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := rulescmd.RulesFile(args)
			if err != nil {
				return err
			}
			e, err := cmdutil.Engine()
			if err != nil {
				return err
			}
			state, err := e.Trust.Check(path)
			if err != nil {
				return err
			}
			if state.Status == rules.Absent {
				return fmt.Errorf("%s does not exist", path)
			}
			if state.Warning != "" {
				fmt.Println(state.Warning)
			}
			fmt.Printf("%s\n%s\n%s", path, strings.Repeat("-", 60), state.Content())
			if !strings.HasSuffix(state.Content(), "\n") {
				fmt.Println()
			}
			fmt.Println(strings.Repeat("-", 60))
			if !yes {
				if !cmdutil.IsTerminal(os.Stdin) {
					return errors.New("not a terminal: review the rules above and re-run with --yes (agents: ask the user first)")
				}
				fmt.Print("These rules can give untracked files in this project a lifespan. Trust them? [y/N] ")
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
					return errors.New("not trusted")
				}
			}
			if _, err := e.Trust.Trust(path, state.SHA256); err != nil {
				return err
			}
			fmt.Println("trusted (sha256 " + state.SHA256[:12] + "…)")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "trust without asking (only after reviewing the rules)")
	return cmd
}
