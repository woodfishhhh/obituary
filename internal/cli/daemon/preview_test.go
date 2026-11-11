//go:build windows

package daemon

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestDaemonDryRun(t *testing.T) {
	for _, tc := range []struct {
		name       string
		installed  bool
		args, want []string
	}{
		{"enable", false, []string{"enable", "--dry-run"}, []string{"create", "every 5 minutes and at sign-in", "--once", "HKCU Run Obituary-", "--tray", "start", "resident obitd"}},
		{"refresh", true, []string{"enable", "--dry-run"}, []string{"update", "every 5 minutes and at sign-in", "unchanged"}},
		{"resident", false, []string{"enable", "--resident", "--dry-run"}, []string{"preserve", "HKCU Run Obituary-", "--tray", "start"}},
		{"disable", true, []string{"disable", "--dry-run"}, []string{"remove", "every 5 minutes and at sign-in", "HKCU Run Obituary-", "--tray", "stop", "resident obitd"}},
		{"disabled", false, []string{"disable", "--dry-run"}, []string{"unchanged", "HKCU Run Obituary-", "resident obitd"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, exe := clitest.DoctorFixture(t)
			home, _ := userdirs.Data()
			config, _ := userdirs.Config()
			plan, err := app.PlanAutostart(runkey.Current, filepath.Join(root, "bin", "obitd.exe"), home, config, true)
			if err != nil {
				t.Fatal(err)
			}
			if tc.installed {
				if err := plan.Apply(); err != nil {
					t.Fatal(err)
				}
			}
			previous := scheduleRemovalPreview
			t.Cleanup(func() { scheduleRemovalPreview = previous })
			scheduleRemovalPreview = func(string, string, string) string {
				if !tc.installed {
					return fmt.Sprintf("unchanged scheduled task %q (absent)", app.TaskName)
				}
				return fmt.Sprintf("remove scheduled task %q (every 5 minutes and at sign-in)\n    installed-obitd.exe --once", app.TaskName)
			}
			cmdutil.RunnerInstalled = func() bool { return tc.installed }
			cmdutil.RunnerRunning = func(string) bool { return tc.installed }
			cmdutil.RunnerTarget = func() (string, error) { return "installed-obitd.exe --once", nil }
			cmdutil.RunnerStart = func(string) error { t.Fatal("dry-run started a process"); return nil }
			clitest.WriteFixture(t, filepath.Join(home, "obitd.stop"), "must remain")
			before := clitest.TreeSnapshot(t, root)
			startup, _ := runkey.Current.Read(plan.Name)
			out, _, err := clitest.Execute(t, NewCmdWithExecutable(exe), tc.args...)
			if err != nil {
				t.Fatal(out, err)
			}
			for _, want := range append(tc.want, app.TaskName, home) {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q: %s", want, out)
				}
			}
			if tc.name == "resident" && strings.Contains(out, "--once") {
				t.Fatal("resident preview changed the schedule", out)
			}
			if tc.name == "disable" && !strings.Contains(out, "installed-obitd.exe --once") {
				t.Fatal("missing existing schedule target", out)
			}
			if tc.name == "disabled" && strings.Contains(out, "remove") {
				t.Fatal("absent runner reported as removed", out)
			}
			after, err := runkey.Current.Read(plan.Name)
			if err != nil || startup != after || !reflect.DeepEqual(before, clitest.TreeSnapshot(t, root)) {
				t.Fatal("dry-run changed startup or filesystem", err)
			}
		})
	}
}

func TestDaemonDryRunPreservesForeignRunValue(t *testing.T) {
	root, exe := clitest.DoctorFixture(t)
	home, _ := userdirs.Data()
	config, _ := userdirs.Config()
	plan, err := app.PlanAutostart(runkey.Current, filepath.Join(root, "bin", "obitd.exe"), home, config, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := runkey.Current.CompareAndSwap(plan.Name, runkey.Value{}, runkey.StringValue("foreign.exe")); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"enable", "disable"} {
		_, _, err := clitest.Execute(t, NewCmdWithExecutable(exe), action, "--dry-run")
		if err == nil || !strings.Contains(err.Error(), "preserving unowned Run value") {
			t.Fatal(err)
		}
		got, _ := runkey.Current.Get(plan.Name)
		if got != "foreign.exe" {
			t.Fatal("foreign startup changed", got)
		}
	}
}
