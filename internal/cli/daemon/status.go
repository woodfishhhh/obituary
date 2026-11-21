package daemon

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

type DaemonStatus struct {
	Mode               string     `json:"mode"` // scheduled, resident, off
	Running            bool       `json:"running"`
	Autostart          string     `json:"autostart,omitempty"`
	Home               string     `json:"home"`
	Tracked            int        `json:"tracked"`
	PendingRecovery    int        `json:"pendingRecovery"`
	RecoveryWarning    string     `json:"recoveryWarning,omitempty"`
	NextDue            *time.Time `json:"nextDue,omitempty"`
	ScheduleInstalled  bool       `json:"scheduleInstalled"`
	ScheduleServesHome bool       `json:"scheduleServesHome"`
	ScheduleTarget     string     `json:"scheduleTarget,omitempty"`
}

func StatusCmd(home func() (string, error)) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon state and the next expiry",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			h, err := home()
			if err != nil {
				return err
			}
			st, err := readStatus(h)
			if err != nil {
				return err
			}
			if asJSON {
				return cmdutil.PrintJSON(st)
			}
			printStatus(st)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func readStatus(home string) (DaemonStatus, error) {
	state, err := (ledger.Store{Home: home}).Read()
	if err != nil {
		return DaemonStatus{}, err
	}
	st := DaemonStatus{Running: cmdutil.RunnerRunning(home), Autostart: cmdutil.RunnerAutostart(), Home: home, Tracked: len(state.Items),
		ScheduleInstalled: cmdutil.RunnerInstalled(), ScheduleServesHome: cmdutil.RunnerServes(home)}
	pending, scanErr := (&core.Service{Store: ledger.Store{Home: home}, FS: winfs.FS{}}).PendingGraves()
	st.PendingRecovery = len(pending)
	if scanErr != nil {
		st.RecoveryWarning = scanErr.Error()
	}
	if st.ScheduleInstalled {
		st.ScheduleTarget, _ = cmdutil.RunnerTarget()
	}
	switch {
	case st.Running || st.Autostart != "":
		st.Mode = "resident"
	case st.ScheduleServesHome:
		st.Mode = "scheduled"
	default:
		st.Mode = "off"
	}
	if next, ok := state.NextDue(); ok {
		st.NextDue = &next
	}
	return st, nil
}

func printStatus(st DaemonStatus) {
	mode := map[string]string{
		"scheduled": fmt.Sprintf(cmdutil.Text("cli.daemon.scheduled_task_every_minutes"), int(app.Interval.Minutes()), app.TaskName),
		"resident":  cmdutil.Text("cli.daemon.resident_tray_daemon_scheduled_fallback_is_reported"),
		"off":       cmdutil.Text("cli.daemon.off_run_obit_daemon_enable"),
	}[st.Mode]
	running := cmdutil.Text("cli.daemon.not_running")
	if st.Running {
		running = cmdutil.Text("cli.daemon.running")
	}
	fmt.Printf(cmdutil.Text("cli.daemon.status"), mode, running, st.Home, st.Tracked)
	fmt.Printf(cmdutil.Text("cli.daemon.pending_recovery"), st.PendingRecovery)
	if st.RecoveryWarning != "" {
		fmt.Fprintln(os.Stderr, cmdutil.Text("cli.warning_prefix"), st.RecoveryWarning)
	}
	if st.ScheduleInstalled && !st.ScheduleServesHome {
		fmt.Printf(cmdutil.Text("cli.daemon.schedule_installed_but_does_not_serve_this"), st.ScheduleTarget)
	}
	if st.NextDue != nil {
		fmt.Printf(cmdutil.Text("cli.daemon.next"), cmdutil.When(*st.NextDue))
	}
}
