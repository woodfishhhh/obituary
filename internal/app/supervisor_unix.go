//go:build linux || darwin

package app

import "github.com/woodfishhhh/obituary/internal/core"

func Supervise(string) error { return core.ErrUnsupported }
func SupervisorProblem(home string) (string, error) {
	if paused, err := Paused(home); err != nil {
		return "", err
	} else if paused {
		return "resident supervision is paused; fallback maintenance is paused too", nil
	}
	s, err := readSupervision(home)
	if err != nil {
		return "", err
	}
	if s.Abandoned {
		return "resident supervision gave up after 3 failures in 15 minutes (" + supervisionDetails(s) + ")", nil
	}
	if s.LaunchFailed {
		return "resident supervisor could not launch the tray (" + supervisionDetails(s) + ")", nil
	}
	if s.Open {
		return "resident supervisor is active (" + supervisionDetails(s) + ")", nil
	}
	if s.Reason == "stopped normally" {
		return "resident daemon was intentionally stopped; fallback schedule remains independent (" + supervisionDetails(s) + ")", nil
	}
	if s.Reason != "" {
		return "resident supervisor exited: " + s.Reason + " (" + supervisionDetails(s) + ")", nil
	}
	return "", nil
}
