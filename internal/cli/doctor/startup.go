//go:build windows

package doctor

import (
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

func diagnoseStartup(daemon, home string) DoctorCheck {
	c := DoctorCheck{Name: "tray-startup", Status: "WARN", Detail: "tray login startup is absent; rerun obit init"}
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
	if command != want {
		c.Detail = "Run command differs from this installation: " + command
		return c
	}
	c.Status, c.Detail = "PASS", "tray starts at login: "+command
	return c
}
