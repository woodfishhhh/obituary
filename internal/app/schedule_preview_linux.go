package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// ScheduleUnit is a read-only preview of one user unit, using the same content
// and ownership checks as installation. Preview never contacts systemd.
type ScheduleUnit struct{ Path, Action, Content string }

func PreviewSchedule(daemon, home, config string, enabled bool) ([]ScheduleUnit, error) {
	files, err := scheduleFiles(daemon, home, config)
	if err != nil {
		return nil, err
	}
	dir, err := userdirs.SystemdUser()
	if err != nil {
		return nil, err
	}
	var units []ScheduleUnit
	for _, name := range []string{serviceName, TaskName} {
		unit, err := previewUnit(dir, name, files[name], enabled)
		if err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, nil
}

func PreviewScheduleRemoval(daemon, home, config string) string {
	units, err := PreviewSchedule(daemon, home, config, false)
	if err != nil {
		return fmt.Sprintf("query failed for scheduled task %q: %v", TaskName, err)
	}
	var lines []string
	for _, unit := range units {
		lines = append(lines, unit.Action+" "+unit.Path)
	}
	return strings.Join(lines, "\n  ")
}

func previewUnit(dir, name, content string, enabled bool) (ScheduleUnit, error) {
	unit := ScheduleUnit{Path: filepath.Join(dir, name), Action: "unchanged"}
	if enabled {
		unit.Action, unit.Content = "create", content
	}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return unit, nil
	}
	if err != nil {
		return unit, err
	}
	defer root.Close()
	if err := ownedUnit(root, name); err != nil {
		return unit, err
	}
	before, err := root.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return unit, nil
	}
	if err != nil {
		return unit, err
	}
	if !enabled {
		unit.Action, unit.Content = "remove", string(before)
	} else {
		unit.Action = "update"
		if string(before) == content {
			unit.Action = "unchanged"
		}
	}
	return unit, nil
}
