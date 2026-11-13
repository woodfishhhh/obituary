package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

type ScheduleUnit struct{ Path, Action, Content string }

func PreviewSchedule(daemon, home, config string, enabled bool) ([]ScheduleUnit, error) {
	dir, err := userdirs.LaunchAgents()
	if err != nil {
		return nil, err
	}
	data, err := launchdContent(daemon, home, config)
	if err != nil {
		return nil, err
	}
	unit := ScheduleUnit{Path: filepath.Join(dir, TaskName+".plist"), Action: "unchanged"}
	if enabled {
		unit.Action, unit.Content = "create", string(data)
	}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []ScheduleUnit{unit}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	before, err := readLaunchd(root)
	if errors.Is(err, os.ErrNotExist) {
		return []ScheduleUnit{unit}, nil
	}
	if err != nil {
		return nil, err
	}
	if !enabled {
		unit.Action, unit.Content = "remove", string(before)
	} else if string(before) == string(data) {
		unit.Action = "unchanged"
	} else {
		unit.Action = "update"
	}
	return []ScheduleUnit{unit}, nil
}

func PreviewScheduleRemoval(daemon, home, config string) string {
	units, err := PreviewSchedule(daemon, home, config, false)
	if err != nil {
		return fmt.Sprintf("query failed for launchd agent %q: %v", TaskName, err)
	}
	return units[0].Action + " " + units[0].Path
}
