//go:build linux || darwin

package app

import "github.com/woodfishhhh/obituary/internal/core"

func Supervise(string) error { return core.ErrUnsupported }
func SupervisorProblem(home string) (string, error) {
	s, err := readSupervision(home)
	if s.Abandoned {
		return "resident supervision gave up after 3 failures in 15 minutes: " + s.Reason, err
	}
	return "", err
}
