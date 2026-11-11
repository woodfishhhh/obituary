//go:build windows

package app

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestReviewRemoveSchedulePreservesForeignTask(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	for _, field := range []string{"executable", "home", "config", "extra action"} {
		t.Run(field, func(t *testing.T) {
			exe, _ := os.Executable()
			home, _ := userdirs.Data()
			daemon := filepath.Join(filepath.Dir(exe), "obitd.exe")
			if field == "executable" {
				daemon = `C:\OtherApp\backup.exe`
			}
			if field == "home" {
				home = `C:\OtherData`
			}
			xml := taskXML(daemon, home, "test", time.Now())
			if field == "config" {
				xml = strings.ReplaceAll(xml, "--config-dir", "--other-config")
			}
			if field == "extra action" {
				xml = strings.ReplaceAll(xml, "</Actions>", "<ComHandler/></Actions>")
			}
			deleted := false
			schtasks = func(args ...string) (string, error) {
				if args[0] == "/Delete" {
					deleted = true
				}
				return xml, nil
			}
			if err := RemoveSchedule(); err == nil || deleted {
				t.Fatalf("foreign task removed or accepted: deleted=%v err=%v", deleted, err)
			}
		})
	}
}

func TestReviewRemoveScheduleReportsQueryFailure(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	schtasks = func(...string) (string, error) { return "Access is denied.", errors.New("exit status 1") }
	if err := RemoveSchedule(); err == nil {
		t.Fatal("task query failure reported as success")
	}
}

func TestTaskXMLRunsHiddenOnBatteryAndCatchesUp(t *testing.T) {
	t.Setenv("OBIT_CONFIG_DIR", `C:\cfg`)
	text := taskXML(`C:\Program Files\obit & co\obitd.exe`, `C:\Users\me\AppData\Local\obituary`, `PC\me`, time.Date(2026, 11, 8, 10, 0, 0, 0, time.Local))
	var task struct {
		Interval  string `xml:"Triggers>TimeTrigger>Repetition>Interval"`
		Logon     string `xml:"Triggers>LogonTrigger>UserId"`
		Battery   bool   `xml:"Settings>DisallowStartIfOnBatteries"`
		StopBatt  bool   `xml:"Settings>StopIfGoingOnBatteries"`
		CatchUp   bool   `xml:"Settings>StartWhenAvailable"`
		Hidden    bool   `xml:"Settings>Hidden"`
		Instances string `xml:"Settings>MultipleInstancesPolicy"`
		Command   string `xml:"Actions>Exec>Command"`
		Arguments string `xml:"Actions>Exec>Arguments"`
	}
	dec := xml.NewDecoder(strings.NewReader(strings.Replace(text, `encoding="UTF-16"`, `encoding="UTF-8"`, 1)))
	if err := dec.Decode(&task); err != nil {
		t.Fatalf("invalid task XML: %v", err)
	}
	if task.Interval != "PT5M" || task.Logon != `PC\me` || task.Battery || task.StopBatt || !task.CatchUp || !task.Hidden || task.Instances != "IgnoreNew" {
		t.Fatalf("unexpected settings: %+v", task)
	}
	if task.Command != `C:\Program Files\obit & co\obitd.exe` || task.Arguments != `--once --home "C:\Users\me\AppData\Local\obituary" --config-dir "C:\cfg"` {
		t.Fatalf("action %q %q", task.Command, task.Arguments)
	}
}

// TestScheduleRoundTrip registers a real (hidden, uniquely named) task and
// removes it again, so it only runs with OBIT_TEST_SCHEDULER=1.
func TestScheduleRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the real Windows Task Scheduler")
	}
	if os.Getenv("OBIT_TEST_SCHEDULER") != "1" {
		t.Skip("set OBIT_TEST_SCHEDULER=1 to touch the real Task Scheduler")
	}
	old, name := TaskName, "Obituary Runner Test "+time.Now().Format("150405")
	TaskName = name
	// Cleanup runs after the test returns: delete this test's task by name,
	// whatever TaskName holds by then, and only then restore it.
	t.Cleanup(func() {
		_, _ = schtasks("/Delete", "/TN", name, "/F")
		TaskName = old
	})

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// DaemonPath looks next to the running binary; the test binary has no
	// obitd.exe beside it, so register the test binary itself as the action.
	home := t.TempDir()
	config, err := userdirs.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := installScheduleFor(exe, home); err != nil {
		t.Fatal(err)
	}
	if !ScheduleInstalled() {
		t.Fatal("task not found after install")
	}
	target, err := ScheduleTarget()
	if err != nil || !strings.Contains(target, "--once --home") {
		t.Fatalf("target %q %v", target, err)
	}
	if err := removeScheduleFor(exe, home, config); err != nil || ScheduleInstalled() {
		t.Fatalf("remove: %v installed=%v", err, ScheduleInstalled())
	}
}

func TestScheduleServesOnlyItsExplicitHome(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	for _, tc := range []struct {
		name, args string
		want       bool
	}{
		{"same", `--once --home "D:\Data Space"`, true},
		{"case and clean", `--home "d:\DATA SPACE\sub\.." --once`, true},
		{"equals", `--home="D:\Data Space"`, true},
		{"other", `--home "D:\Elsewhere"`, false},
		{"prefix", `--home "D:\Data Space extra"`, false},
		{"implicit", `--once`, false},
		{"wrong flag", `--config-dir "D:\Data Space"`, false},
		{"missing value", `--once --home`, false},
		{"empty", `--home ""`, false},
		{"duplicate", `--home "D:\Data Space" --home "D:\Other"`, false},
		{"terminator", `-- --home "D:\Data Space"`, false},
		{"another flag value", `--config-dir --home "D:\Data Space"`, false},
		{"invalid flag", `--unknown --home "D:\Data Space"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schtasks = func(args ...string) (string, error) {
				if strings.Join(args, " ") != "/Query /TN "+TaskName+" /XML" {
					t.Fatalf("unexpected scheduler operation: %v", args)
				}
				return fmt.Sprintf(`<Task><Triggers><TimeTrigger/></Triggers><Actions><Exec><Command>D:\Program Files\obitd.exe</Command><Arguments>%s</Arguments></Exec></Actions></Task>`, tc.args), nil
			}
			if got := ScheduleServes(`D:\Data Space`); got != tc.want {
				t.Fatalf("serves = %v, want %v", got, tc.want)
			}
		})
	}
	for _, output := range []string{"", "<Task"} {
		schtasks = func(...string) (string, error) { return output, nil }
		if ScheduleServes(`D:\Data Space`) {
			t.Fatal("invalid task claims to serve home")
		}
	}
	schtasks = func(...string) (string, error) { return "", errors.New("missing") }
	if ScheduleServes(`D:\Data Space`) {
		t.Fatal("absent task claims to serve home")
	}
}

func TestScheduleTargetPreservesExecutableWithSpaces(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	schtasks = func(...string) (string, error) {
		return taskXML(`D:\Program Files\obitd.exe`, `D:\Data Space`, "test", time.Now()), nil
	}
	target, err := ScheduleTarget()
	if err != nil {
		t.Fatal(err)
	}
	args, err := windows.DecomposeCommandLine(target)
	if err != nil || len(args) < 4 || args[0] != `D:\Program Files\obitd.exe` || args[3] != `D:\Data Space` {
		t.Fatalf("target parsed as %q: %v", args, err)
	}
}

func TestScheduleServesRequiresEnabledTaskAndTrigger(t *testing.T) {
	old := schtasks
	t.Cleanup(func() { schtasks = old })
	for _, tc := range []struct {
		name, settings, triggers string
		want                     bool
	}{
		{"defaults", "", "<TimeTrigger/>", true},
		{"task disabled", "<Enabled>false</Enabled>", "<TimeTrigger/>", false},
		{"all disabled", "<Enabled>true</Enabled>", "<TimeTrigger><Enabled>false</Enabled></TimeTrigger><LogonTrigger><Enabled>false</Enabled></LogonTrigger>", false},
		{"one enabled", "", "<TimeTrigger><Enabled>false</Enabled></TimeTrigger><LogonTrigger><Enabled>true</Enabled></LogonTrigger>", true},
		{"one default", "", "<TimeTrigger><Enabled>false</Enabled></TimeTrigger><LogonTrigger/>", true},
		{"no triggers", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schtasks = func(args ...string) (string, error) {
				if strings.Join(args, " ") != "/Query /TN "+TaskName+" /XML" {
					t.Fatalf("unexpected scheduler operation: %v", args)
				}
				return `<Task><Settings>` + tc.settings + `</Settings><Triggers>` + tc.triggers + `</Triggers><Actions><Exec><Command>D:\obitd.exe</Command><Arguments>--once --home D:\data</Arguments></Exec></Actions></Task>`, nil
			}
			if got := ScheduleServes(`D:\data`); got != tc.want {
				t.Errorf("serves=%v, want %v", got, tc.want)
			}
		})
	}
}
