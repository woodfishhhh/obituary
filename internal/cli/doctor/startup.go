//go:build windows

package doctor

import (
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

func (lang diagnostics) diagnoseStartup(daemon, home string) DoctorCheck {
	c := DoctorCheck{Name: "tray-startup", Status: "WARN", Detail: lang.Text("cli.doctor.tray_login_startup_is_absent_rerun_obit", nil)}
	command := cmdutil.RunnerAutostart()
	if command == "" {
		return c
	}
	config, err := userdirs.Config()
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		return c
	}
	want := windows.ComposeCommandLine([]string{daemon, "--tray", "--home", home, "--config-dir", config})
	supervised := windows.ComposeCommandLine([]string{daemon, "--supervise", "--home", home, "--config-dir", config})
	if command != want && command != supervised {
		c.Detail = lang.Text("cli.doctor.run_command_differs_from_this_installation", nil) + command
		return c
	}
	c.Status, c.Detail = "PASS", lang.Text("cli.doctor.tray_starts_at_login", nil)+command
	return c
}
