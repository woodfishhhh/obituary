//go:build windows

package doctor

import (
	"testing"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"golang.org/x/sys/windows"
)

func TestDoctorChecksTrayStartupCommand(t *testing.T) {
	t.Setenv("OBIT_CONFIG_DIR", `C:\config`)
	old := cmdutil.RunnerAutostart
	t.Cleanup(func() { cmdutil.RunnerAutostart = old })
	for _, tc := range []struct{ command, status string }{
		{"", "WARN"}, {"unrelated.exe", "WARN"},
		{windows.ComposeCommandLine([]string{`C:\obitd.exe`, "--tray", "--home", `C:\data`, "--config-dir", `C:\config`}), "PASS"},
	} {
		cmdutil.RunnerAutostart = func() string { return tc.command }
		if got := diagnoseStartup(`C:\obitd.exe`, `C:\data`); got.Status != tc.status {
			t.Fatal(got)
		}
	}
}
