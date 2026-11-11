package app

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

// RemoveSchedule removes only this installation's explicitly pinned runner.
func RemoveSchedule() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := userdirs.Data()
	if err != nil {
		return err
	}
	config, err := userdirs.Config()
	if err != nil {
		return err
	}
	return removeScheduleFor(filepath.Join(filepath.Dir(exe), daemonName), home, config)
}

func removeScheduleFor(daemon, home, config string) error {
	definition, err := removalScheduleXML()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !ownedSchedule(definition, daemon, home, config) {
		return fmt.Errorf("scheduled task %q is not owned by this installation; preserved", TaskName)
	}
	current, err := removalScheduleXML()
	if err != nil || current != definition {
		return errors.Join(err, errors.New("scheduled task changed before removal; preserved"))
	}
	if out, err := schtasks("/Delete", "/TN", TaskName, "/F", "/HRESULT"); err != nil {
		return fmt.Errorf("schtasks /Delete: %w: %s", err, out)
	}
	// Task Scheduler has no compare-and-delete API. Re-query after the delete:
	// a concurrently registered replacement must never be deleted in a retry,
	// nor reported as a successful uninstall allowing the installer to proceed.
	_, err = removalScheduleXML()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return errors.Join(err, errors.New("scheduled task remains or was replaced during removal; inspect it before retrying"))
}

func removalScheduleXML() (string, error) {
	// /HRESULT provides locale-independent errors, unlike matching schtasks'
	// translated console text. Only ERROR_FILE_NOT_FOUND proves absence.
	out, err := schtasks("/Query", "/TN", TaskName, "/XML", "/HRESULT")
	if err == nil {
		return out, nil
	}
	var status interface{ ExitCode() int }
	if errors.As(err, &status) && uint32(status.ExitCode()) == 0x80070002 {
		return "", os.ErrNotExist
	}
	return "", fmt.Errorf("query scheduled task %q: %w: %s", TaskName, err, out)
}

// PreviewScheduleRemoval uses exactly the execution ownership predicate.
func PreviewScheduleRemoval(daemon, home, config string) string {
	definition, err := removalScheduleXML()
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Sprintf("unchanged scheduled task %q (absent)", TaskName)
	}
	if err != nil {
		return fmt.Sprintf("query failed for scheduled task %q: %v", TaskName, err)
	}
	if ownedSchedule(definition, daemon, home, config) {
		return fmt.Sprintf("remove scheduled task %q (every %d minutes and at sign-in)\n    %s", TaskName, int(Interval.Minutes()), scheduleDefinitionTarget(definition))
	}
	return fmt.Sprintf("preserve scheduled task %q: task belongs to another installation (path, home, config)\n    %s", TaskName, scheduleDefinitionTarget(definition))
}

func scheduleDefinitionTarget(definition string) string {
	var task struct {
		Actions struct {
			Exec []struct{ Command, Arguments string }
		}
	}
	decoder := xml.NewDecoder(strings.NewReader(strings.Replace(definition, `encoding="UTF-16"`, `encoding="UTF-8"`, 1)))
	if err := decoder.Decode(&task); err != nil {
		return "invalid task definition: " + err.Error()
	}
	var targets []string
	for _, action := range task.Actions.Exec {
		targets = append(targets, action.Command+" "+action.Arguments)
	}
	return strings.Join(targets, "; ")
}

func ownedSchedule(definition, daemon, home, config string) bool {
	var task struct {
		Actions struct {
			Entries []struct {
				XMLName          xml.Name
				Command          string
				Arguments        string
				WorkingDirectory string
			} `xml:",any"`
		}
	}
	// Decode every action so additional Exec/COM actions cannot be hidden.
	decoder := xml.NewDecoder(strings.NewReader(strings.Replace(definition, `encoding="UTF-16"`, `encoding="UTF-8"`, 1)))
	if decoder.Decode(&task) != nil || len(task.Actions.Entries) != 1 {
		return false
	}
	action := task.Actions.Entries[0]
	if action.XMLName.Local != "Exec" {
		return false
	}
	if !strings.EqualFold(filepath.Clean(action.Command), filepath.Clean(daemon)) || action.WorkingDirectory != "" {
		return false
	}
	args, err := windows.DecomposeCommandLine("obitd " + action.Arguments)
	want := []string{"obitd", "--once", "--home", home, "--config-dir", config}
	return err == nil && slices.EqualFunc(args, want, strings.EqualFold)
}
