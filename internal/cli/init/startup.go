package init

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func planStartup(self cmdutil.Executable, enabled bool) (*app.AutostartPlan, error) {
	plan, err := readStartupPlan(self, enabled)
	if err != nil || plan == nil {
		return plan, err
	}
	if plan.Pending() {
		action := "set"
		if !enabled {
			action = "remove"
		}
		fmt.Printf("  %-9s HKCU Run %s %s\n", cmdutil.Column(cmdutil.Localizer(), cmdutil.Localizer().Label(action)), plan.Name, plan.After)
	}
	return plan, nil
}

func readStartupPlan(self cmdutil.Executable, enabled bool) (*app.AutostartPlan, error) {
	if runtime.GOOS != "windows" {
		return nil, nil
	}
	exe, err := self.Path()
	if err != nil {
		return nil, err
	}
	daemon := filepath.Join(filepath.Dir(exe), "obitd.exe")
	if _, err := os.Stat(daemon); enabled && errors.Is(err, os.ErrNotExist) {
		fmt.Println(cmdutil.Text("cli.init.tray_startup_unavailable_obitd_exe_is_missing"))
		return nil, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	home, err := userdirs.Data()
	if err != nil {
		return nil, err
	}
	config, err := userdirs.Config()
	if err != nil {
		return nil, err
	}
	return app.PlanAutostart(runkey.Current, daemon, home, config, enabled)
}
