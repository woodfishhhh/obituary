package add

import (
	"errors"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/core/lifespan"
)

func NewCmd() *cobra.Command {
	var (
		at, note               string
		force, asJSON          bool
		noDaemon, dockerVolume bool
	)
	cmd := &cobra.Command{
		Use:     "add <path>... <duration> | add <path>... --at <time>",
		Aliases: []string{"mark"},
		Short:   "Give paths a lifespan (re-adding a tracked path resets its expiry)",
		Example: `  obit add debug.log 2h
  obit add output\shots 1d12h
  obit add report.zip --at "2026-12-01 18:00"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, expires, err := parseExpiry(args, at, time.Now(), asJSON)
			if err != nil {
				return err
			}
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			var results []lifecycle.Result
			if dockerVolume {
				results, err = svc.AddDockerVolume(paths, lifecycle.AddOptions{ExpiresAt: expires, Force: force, Note: note})
			} else {
				results, err = svc.Add(paths, lifecycle.AddOptions{ExpiresAt: expires, Force: force, Note: note})
			}
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			reportErr := cmdutil.Report(results, asJSON)
			if !noDaemon && cmdutil.AnyOK(results) {
				cmdutil.EnsureDaemon(svc.Store.Home, asJSON)
			}
			return reportErr
		},
	}
	cmd.Flags().StringVar(&at, "at", "", `expire at a local time: "2006-01-02 15:04", "2006-01-02" or "15:04"`)
	cmd.Flags().BoolVar(&force, "force", false, "allow folders that contain a .git repository")
	cmd.Flags().StringVar(&note, "note", "", "free-text note shown in the list")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&noDaemon, "no-daemon", false, "do not start the background daemon")
	cmd.Flags().BoolVar(&dockerVolume, "docker-volume", false, "treat operands as Docker volume names")
	return cmd
}

func parseExpiry(args []string, at string, now time.Time, asJSON bool) ([]string, time.Time, error) {
	if at != "" {
		expires, err := lifespan.ParseAt(at, now)
		return args, expires, err
	}
	if len(args) < 2 {
		return nil, time.Time{}, errors.New(cmdutil.OutputLocalizer(asJSON).Text("cli.add.give_a_duration_after_the_paths_for", nil))
	}
	d, err := lifespan.ParseDuration(args[len(args)-1])
	if err != nil {
		return nil, time.Time{}, err
	}
	return args[:len(args)-1], now.Add(d), nil
}
