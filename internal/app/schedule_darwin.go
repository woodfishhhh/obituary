package app

import (
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

var TaskName = "obituary"
var ErrScheduleDisabled = fmt.Errorf("launchd runner is unavailable: %w", core.ErrUnsupported)

const Interval = 5 * time.Minute

// TODO: implement owned, user-level launchd agents.
func InstallSchedule(string) error                         { return ErrScheduleDisabled }
func RemoveSchedule() error                                { return ErrScheduleDisabled }
func ScheduleInstalled() bool                              { return false }
func ScheduleTarget() (string, error)                      { return "", ErrScheduleDisabled }
func ScheduleServes(string) bool                           { return false }
func PreviewScheduleRemoval(string, string, string) string { return ErrScheduleDisabled.Error() }
