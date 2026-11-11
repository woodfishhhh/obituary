package app

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// TaskName is the per-user scheduled task that runs `obitd --once`.
var TaskName = `Obituary Runner`

// ErrScheduleDisabled means the task cannot run from an enabled trigger.
var ErrScheduleDisabled = errors.New("scheduled task is disabled")

// Interval is how often the scheduled runner wakes up.
const Interval = 5 * time.Minute

// taskXML builds the Task Scheduler definition. The XML form is needed
// because schtasks' flags cannot express what an invisible background chore
// requires: keep running on battery, catch up after sleep (StartWhenAvailable),
// never stack instances, and stay hidden.
func taskXML(daemon, home, userID string, start time.Time) string {
	esc := func(s string) string {
		var b strings.Builder
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	args := "--once"
	pinned := daemonArgs(home)
	for i := 0; i+1 < len(pinned); i += 2 {
		args += fmt.Sprintf(` %s "%s"`, pinned[i], pinned[i+1])
	}
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Obituary: buries items whose lifespan ran out and purges the graveyard. Runs hidden every ` + fmt.Sprint(int(Interval.Minutes())) + ` minutes.</Description>
    <URI>\` + esc(TaskName) + `</URI>
  </RegistrationInfo>
  <Triggers>
    <TimeTrigger>
      <Repetition>
        <Interval>PT` + fmt.Sprint(int(Interval.Minutes())) + `M</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
      <StartBoundary>` + start.Format("2006-01-02T15:04:05") + `</StartBoundary>
      <Enabled>true</Enabled>
    </TimeTrigger>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>` + esc(userID) + `</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>` + esc(userID) + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>true</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT30M</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + esc(daemon) + `</Command>
      <Arguments>` + esc(args) + `</Arguments>
    </Exec>
  </Actions>
</Task>
`
}

var schtasks = func(args ...string) (string, error) {
	cmd := exec.Command("schtasks", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// InstallSchedule registers (or replaces) the scheduled runner for home.
func InstallSchedule(home string) error {
	daemon, err := DaemonPath()
	if err != nil {
		return err
	}
	return installScheduleFor(daemon, home)
}

func installScheduleFor(daemon, home string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	text := taskXML(daemon, home, u.Username, time.Now().Add(time.Minute))
	// Task Scheduler wants the file in UTF-16 with a byte order mark.
	units := utf16.Encode([]rune(text))
	buf := make([]byte, 2+2*len(units))
	buf[0], buf[1] = 0xFF, 0xFE
	for i, u := range units {
		buf[2+2*i], buf[3+2*i] = byte(u), byte(u>>8)
	}
	tmp, err := os.CreateTemp("", "obit-task-*.xml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // our own temporary file
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if out, err := schtasks("/Create", "/TN", TaskName, "/XML", tmpPath, "/F"); err != nil {
		return fmt.Errorf("schtasks /Create: %v: %s", err, out) //nolint:errorlint // schtasks exit errors are diagnostics, not the scheduler API's error types.
	}
	return nil
}

// ScheduleInstalled reports whether the scheduled runner exists.
func ScheduleInstalled() bool {
	_, err := schtasks("/Query", "/TN", TaskName)
	return err == nil
}

// ScheduleTarget returns the task's command line, with ErrScheduleDisabled
// when the task or all its triggers are disabled (and "" if absent).
func ScheduleTarget() (string, error) {
	out, err := schtasks("/Query", "/TN", TaskName, "/XML")
	if err != nil {
		return "", errors.New("not installed")
	}
	var task struct {
		Command   string `xml:"Actions>Exec>Command"`
		Arguments string `xml:"Actions>Exec>Arguments"`
		Enabled   *bool  `xml:"Settings>Enabled"`
		Triggers  struct {
			Entries []struct {
				Enabled *bool `xml:"Enabled"`
			} `xml:",any"`
		} `xml:"Triggers"`
	}
	// schtasks prints the XML declaration as UTF-16 but the text is ours to parse.
	decoder := xml.NewDecoder(strings.NewReader(strings.Replace(out, `encoding="UTF-16"`, `encoding="UTF-8"`, 1)))
	if err := decoder.Decode(&task); err != nil {
		return "", err
	}
	target := windows.EscapeArg(filepath.Clean(task.Command)) + " " + task.Arguments
	if task.Enabled != nil && !*task.Enabled {
		return target, ErrScheduleDisabled
	}
	for _, trigger := range task.Triggers.Entries {
		if trigger.Enabled == nil || *trigger.Enabled {
			return target, nil
		}
	}
	return target, ErrScheduleDisabled
}

// ScheduleServes reports whether the installed task explicitly serves home.
func ScheduleServes(home string) bool {
	target, err := ScheduleTarget()
	if err != nil {
		return false
	}
	args, err := windows.DecomposeCommandLine(target)
	if err != nil || len(args) == 0 {
		return false
	}
	// Use the same parser and flags as obitd, including -- and flag values
	// containing spaces; a --home appearing inside another value is not a pin.
	flags := flag.NewFlagSet("obitd", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	pinned := flags.String("home", "", "")
	flags.String("config-dir", "", "")
	flags.Bool("once", false, "")
	if flags.Parse(args[1:]) != nil {
		return false
	}
	return *pinned != "" && strings.EqualFold(filepath.Clean(*pinned), filepath.Clean(home))
}
